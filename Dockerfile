FROM golang:latest 
COPY . /app/ 
WORKDIR /app 
RUN touch config.yaml
RUN go build -o main .
CMD ["/app/main", "config.yaml"]
