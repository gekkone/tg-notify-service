module TgNotify

go 1.19

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/mattn/go-sqlite3 v1.14.15
)

replace github.com/go-telegram-bot-api/telegram-bot-api/v5 => ./telegram-bot-api/
