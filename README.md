# gin-demo

Simple Gin HTTP service with register/login and leveled file logging using Logrus + lfshook + lumberjack.

Quick start

1. Ensure Go is installed.
2. From project root run:

```bash
go mod tidy
go run main.go
```

3. Endpoints:

- `POST /register` JSON {"username":"...","password":"..."}
- `POST /login` JSON {"username":"...","password":"..."}

Logs are written to `./logs/info.log`, `./logs/warn.log`, `./logs/error.log`.
