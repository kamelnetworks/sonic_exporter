# Use Prometheus' Golang Builder to avoid depending on Docker Hub
FROM quay.io/prometheus/golang-builder:1.17-base as builder

WORKDIR /build

COPY . .
RUN go get -v -t -d ./...
RUN make build

FROM scratch
WORKDIR /opt/sonic_exporter

COPY --from=builder /build/target/sonic_exporter .
COPY cli/*.py /cli/

EXPOSE 9893
ENTRYPOINT ["./sonic_exporter"]
CMD []
