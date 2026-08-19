FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /sidesave-relay ./cmd/sidesave-relay

FROM alpine:3.20
RUN adduser -D relay
USER relay
COPY --from=build /sidesave-relay /usr/local/bin/sidesave-relay
ENV PORT=10000
EXPOSE 10000
CMD ["sidesave-relay"]
