## Vaccination telegram bot

Bot watches for gorzdrav.spb.ru for the vaccination tickets.

```
$ go build ./cmd/vaccibot

$ vaccibot --help
Usage of ./vaccibot:
  -chat string
        bot chat id(with -100 before chatID)
  -check_every duration
        check interval (default 10m0s)
  -db string
        db files path (default "/tmp/nutsdb")
  -filter string
        regexp to filter hospital (default ".*")
  -rps int
        rate limit (default 2)
  -send_every duration
        send diffs interval (default 30m0s)
  -token string
        telegram token
```
