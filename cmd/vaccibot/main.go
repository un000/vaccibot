package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/un000/vaccibot/cmd/vaccibot/gorzdrav"
	"github.com/un000/vaccibot/cmd/vaccibot/telegram"
	"github.com/un000/vaccibot/cmd/vaccibot/wait"
)

var (
	flagToken  = flag.String("token", "", "telegram token")
	flagChatID = flag.String("chat", "", "bot chat id(with -100 before chatID)")
)

func main() {
	flag.Parse()
	if *flagToken == "" || *flagChatID == "" {
		log.Fatal("-token or chat are empty")
	}

	sender := telegram.NewSender(*flagToken, *flagChatID)
	ctx := notifyExit(context.Background())
	wg := &wait.Group{}
	client := gorzdrav.NewClient()

	districts := make(chan *gorzdrav.District)
	lpus := make(chan *gorzdrav.LPU)
	specialties := make(chan *gorzdrav.Specialty)

	wg.Add(func() {
		ticker := time.NewTicker(7 * time.Minute)
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
	wg.AddMany(1, func() {
		for lpu := range lpus {
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
		alreadyChecked := make(map[checkedKey]checkedValues, 512)
		covidRegexp := regexp.MustCompile("(?i)(COVID)|(вакцин)/(ковид)")
		for specialty := range specialties {
			if !covidRegexp.MatchString(specialty.Name) {
				continue
			}

			url := "https://gorzdrav.spb.ru/service-covid-vaccination-schedule#%5B%7B%22district%22:%22" + specialty.LPU.District.ID + "%22%7D,%7B%22lpu%22:%22" + strconv.Itoa(specialty.LPU.ID) + "%22%7D,%7B%22speciality%22:%22" + specialty.ID + "%22%7D%5D"

			key := checkedKey{
				DistrictID:  specialty.LPU.District.ID,
				LPUID:       specialty.LPU.ID,
				SpecialtyID: specialty.ID,
			}
			newValue := checkedValues{
				CountFreeParticipant: specialty.CountFreeParticipant,
				CountFreeTicket:      specialty.CountFreeTicket,
			}
			if checked, ok := alreadyChecked[key]; ok {
				if reflect.DeepEqual(checked, newValue) {
					log.Printf("INFO already sent %s", url)
					continue
				}
			}
			alreadyChecked[key] = newValue

			if specialty.CountFreeTicket > 0 || specialty.CountFreeParticipant > 0 {
				message := fmt.Sprintf(
					messageFMT,
					specialty.LPU.District.Name,
					specialty.LPU.LpuShortName,
					specialty.LPU.Address,
					specialty.Name,
					url,
					specialty.CountFreeTicket,
					specialty.CountFreeParticipant,
				)
				if err := sender.SendMessage(message); err != nil {
					log.Printf("ERROR %s", err)
				}
				if err := sender.SendLocation(specialty.LPU.Latitide, specialty.LPU.Longitude); err != nil {
					log.Printf("ERROR %s", err)
				}
			}
		}
	})

	wg.Wait(ctx)
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

type checkedKey struct {
	DistrictID  string
	LPUID       int
	SpecialtyID string
}
type checkedValues struct {
	CountFreeParticipant int
	CountFreeTicket      int
}

const messageFMT = `#%s

🏥 %s
🗺 %s

[Записаться %s](%s)
Всего: *%d* 🎫
*Доступных*: *%d* 🎫
`
