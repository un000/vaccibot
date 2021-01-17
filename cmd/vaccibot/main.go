package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/un000/vaccibot/cmd/vaccibot/gorzdrav"
	"github.com/un000/vaccibot/cmd/vaccibot/telegram"
	"github.com/un000/vaccibot/cmd/vaccibot/wait"
	"github.com/xujiajun/nutsdb"
)

var (
	flagToken      = flag.String("token", "", "telegram token")
	flagChatID     = flag.String("chat", "", "bot chat id(with -100 before chatID)")
	flagDB         = flag.String("db", "/tmp/nutsdb", "db files path")
	flagCheckEvery = flag.Duration("check_every", 10*time.Minute, "check interval")
	flagSendEvery  = flag.Duration("send_every", 30*time.Minute, "send diffs interval")
	flagRPS        = flag.Int64("rps", 2, "rate limit")
)

const (
	bucket     = "gorzdrav_sent"
	timeLayout = "2006-01-02 15:04:05"
)

func main() {
	flag.Parse()
	if *flagToken == "" || *flagChatID == "" {
		log.Fatal("-token or chat are empty")
	}
	ctx := notifyExit(context.Background())

	opt := nutsdb.DefaultOptions
	opt.Dir = *flagDB
	db, err := nutsdb.Open(opt)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Panicf("error closing db: %s", err)
		}
	}()
	if err := db.Update(func(tx *nutsdb.Tx) error {
		return tx.Put(bucket, []byte("ping"), []byte("pong"), 0)
	}); err != nil {
		log.Panicf("ERROR creating bucket: %s", err)
	}

	sender := telegram.NewSender(*flagToken, *flagChatID)
	wg := &wait.Group{}
	client := gorzdrav.NewClient(*flagRPS)

	districts := make(chan *gorzdrav.District)
	lpus := make(chan *gorzdrav.LPU)
	specialties := make(chan *gorzdrav.Specialty)

	wg.Add(func() {
		ticker := time.NewTicker(*flagCheckEvery)
		defer ticker.Stop()

		f := func() {
			log.Println("INFO scan started")
			res, err := client.GetDistricts(ctx)
			if err != nil {
				log.Printf("ERROR %s", err)
				return
			}
			for _, district := range res {
				districts <- district
			}
			log.Println("INFO scan done")
		}

		f()
		for range ticker.C {
			if ctx.Err() != nil {
				return
			}
			f()
		}
		close(districts)
	})
	wg.Add(func() {
		for district := range districts {
			res, err := client.GetLPUs(ctx, district)
			if err != nil {
				log.Printf("ERROR %s", err)
				continue
			}
			for _, lpu := range res {
				lpus <- lpu
			}
		}
	})
	wg.Add(func() {
		for lpu := range lpus {
			if lpu.ID == 182 || lpu.ID == 319 {
				log.Printf("WARN skipping bad LPU %s", lpu.LpuShortName)
				continue
			}

			res, err := client.GetSpecialties(ctx, lpu)
			if err != nil {
				log.Printf("ERROR %s", err)
				continue
			}
			for _, specialty := range res {
				specialties <- specialty
			}
		}
	})
	wg.Add(func() {
		covidRegexp := regexp.MustCompile("(?i)(COVID)|(вакцин)/(ковид)")
		for specialty := range specialties {
			if !covidRegexp.MatchString(specialty.Name) {
				continue
			}

			ok, err := needToSend(db, specialty)
			if err != nil {
				log.Printf("ERROR checking need to send: %s", err)
				continue
			}
			if !ok {
				log.Printf("INFO skipping, already sent: %s", specialty.LPU.LpuShortName)
				continue
			}
			if err := send(sender, specialty); err != nil {
				log.Printf("ERROR sending: %s", err)
				continue
			}
			if err := markSent(db, specialty); err != nil {
				log.Printf("ERROR marking sent: %s", err)
				continue
			}
			log.Printf("INFO sent %s [%d %d]", specialty.LPU.LpuShortName, specialty.CountFreeTicket, specialty.CountFreeParticipant)
		}
	})

	wg.Wait(ctx)
}

func send(sender *telegram.Sender, specialty *gorzdrav.Specialty) error {
	message := fmt.Sprintf(
		messageFMT,
		specialty.LPU.District.Name,
		fmt.Sprintf("%s%d%s", specialty.LPU.District.ID, specialty.LPU.ID, specialty.ID),
		specialty.LPU.LpuShortName,
		specialty.LPU.Address,
		specialty.LPU.Phone,
		specialty.LPU.Email,
		specialty.Name,
		"https://gorzdrav.spb.ru/service-covid-vaccination-schedule#%5B%7B%22district%22:%22"+specialty.LPU.District.ID+"%22%7D,%7B%22lpu%22:%22"+strconv.Itoa(specialty.LPU.ID)+"%22%7D,%7B%22speciality%22:%22"+specialty.ID+"%22%7D%5D",
		specialty.CountFreeTicket,
		specialty.CountFreeParticipant,
	)
	if err := sender.SendMessage(message); err != nil {
		return fmt.Errorf("error sending telegram message: %w", err)
	}
	_ = sender.SendLocation(specialty.LPU.Latitide, specialty.LPU.Longitude)
	return nil
}

func needToSend(db *nutsdb.DB, specialty *gorzdrav.Specialty) (bool, error) {
	var entry *nutsdb.Entry
	if err := db.View(func(tx *nutsdb.Tx) (err error) {
		entry, err = tx.Get(bucket, sentKey{
			DistrictID:  specialty.LPU.District.ID,
			LPUID:       specialty.LPU.ID,
			SpecialtyID: specialty.ID,
		}.Marshal())
		if errors.Is(err, nutsdb.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("error getting key: %w", err)
		}
		return nil
	}); err != nil {
		return false, fmt.Errorf("error viewing bucket: %w", err)
	}
	if entry == nil {
		return true, nil
	}

	oldValue := sentValue{}
	if err := json.Unmarshal(entry.Value, &oldValue); err != nil {
		return false, fmt.Errorf("error unmarshaling value: %w", err)
	}

	lastSend, err := time.ParseInLocation("2006-01-02 15:04:05", oldValue.LastSend, time.UTC)
	if err != nil {
		return false, fmt.Errorf("error parsing last send %s: %w", oldValue.LastSend, err)
	}
	if time.Now().UTC().After(lastSend.Add(*flagSendEvery)) {
		if oldValue.CountFreeTicket != specialty.CountFreeTicket ||
			oldValue.CountFreeParticipant != specialty.CountFreeParticipant {
			return true, nil
		}
	}

	return false, nil
}

func markSent(db *nutsdb.DB, specialty *gorzdrav.Specialty) error {
	if err := db.Update(func(tx *nutsdb.Tx) error {
		if err := tx.Put(
			bucket,
			sentKey{
				DistrictID:  specialty.LPU.District.ID,
				LPUID:       specialty.LPU.ID,
				SpecialtyID: specialty.ID,
			}.Marshal(),
			sentValue{
				CountFreeParticipant: specialty.CountFreeParticipant,
				CountFreeTicket:      specialty.CountFreeTicket,
				LastSend:             time.Now().UTC().Format(timeLayout),
			}.Marshal(),
			0,
		); err != nil {
			return fmt.Errorf("error saving key: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("error commiting transaction: %w", err)
	}

	return nil
}

func notifyExit(ctx context.Context) context.Context {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-sigChan
		cancel()
	}()

	return ctx
}

type sentKey struct {
	DistrictID  string
	LPUID       int
	SpecialtyID string
}

func (k sentKey) Marshal() []byte {
	b, _ := json.Marshal(k)
	return b
}

type sentValue struct {
	CountFreeParticipant int
	CountFreeTicket      int
	LastSend             string
}

func (v sentValue) Marshal() []byte {
	b, _ := json.Marshal(v)
	return b
}

const messageFMT = `#%s
🔎 #%s

🏥 %s
🗺 %s
☎️ %s
📧 %s

[Записаться %s](%s)
Всего *%d* 🎫
Доступных *%d* 🎫
`
