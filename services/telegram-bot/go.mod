module github.com/conveer/telegram-bot

go 1.21

require (
	github.com/go-telegram/bot v1.1.1
	go.mongodb.org/mongo-driver v1.12.1
	google.golang.org/grpc v1.58.2
	github.com/joho/godotenv v1.5.1
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/mattn/go-sqlite3 v1.14.17
	github.com/guptarohit/asciigraph v0.5.5
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/conveer/pkg => ../../pkg