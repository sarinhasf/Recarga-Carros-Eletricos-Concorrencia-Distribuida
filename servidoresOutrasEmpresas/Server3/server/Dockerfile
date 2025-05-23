FROM golang:1.20-alpine

WORKDIR /app
COPY . .

RUN ln -s /Server3/dados/dadosEmpresas.json ./dadosEmpresas.json


RUN go mod tidy
RUN go build -o server .
CMD ["./server"]
