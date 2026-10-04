FROM --platform=$BUILDPLATFORM golang:1.26.5-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY pkg ./pkg
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -buildid=" -o /out/kyverno-nono ./cmd/kyverno-nono

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=build /out/kyverno-nono /kyverno-nono
USER 65532:65532
EXPOSE 8765 8080
ENTRYPOINT ["/kyverno-nono"]
CMD ["serve"]
