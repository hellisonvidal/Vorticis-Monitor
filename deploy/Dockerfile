FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/vorticis-monitor ./cmd/vorticis-monitor

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 vorticis
ENV VORTICIS_DATA=/dados VORTICIS_HOST=0.0.0.0 TZ=America/Sao_Paulo
COPY --from=build /out/vorticis-monitor /usr/local/bin/vorticis-monitor
USER vorticis
VOLUME /dados
EXPOSE 8765
HEALTHCHECK CMD wget -qO- http://127.0.0.1:8765/api/v1/saude || exit 1
ENTRYPOINT ["vorticis-monitor", "run"]
