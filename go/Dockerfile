ARG builder_image=golang:alpine
ARG base_image=golang:alpine


###########################


FROM ${builder_image} AS builder
WORKDIR /src
COPY . .
RUN go build -o ../go-prototype .


FROM ${base_image}
COPY --from=builder /go-prototype /bin
ENTRYPOINT ["go-prototype"]
