# Build from the committed generated templates, queries, CSS and browser assets.
FROM golang:1.27.3-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY cmd ./cmd
COPY content ./content
COPY db ./db
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/komanook ./cmd/komanook
RUN mkdir -p /out/media

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/komanook /komanook
COPY --from=build --chown=65532:65532 /out/media /var/lib/komanook
COPY licenses /licenses
ENV APP_ENV=production HTTP_ADDR=0.0.0.0:8080 MEDIA_DIR=/var/lib/komanook
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/komanook"]
CMD ["serve"]
