FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/server .

FROM scratch
COPY --from=build /out/server /server
ENV LISTEN_ADDR=0.0.0.0:8080
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/server"]
