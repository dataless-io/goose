FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/brokerd ./cmd/brokerd

FROM gcr.io/distroless/static-debian12
WORKDIR /

COPY --from=build /out/brokerd /brokerd

EXPOSE 8081
ENTRYPOINT ["/brokerd"]
