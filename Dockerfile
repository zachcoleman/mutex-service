FROM golang:alpine3.19 as builder

WORKDIR /app
COPY . .
# Note: -ldflags="-w -s" helps shrinks the binary size
RUN export CGO_ENABLED=0; go build -o main -ldflags="-w -s" .  

FROM scratch as runner
WORKDIR /app
COPY --from=builder /app/main /app/main
CMD [ "/app/main" ]