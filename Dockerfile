# ── Build stage ───────────────────────────────────────────────────────────────
# Run the build toolchain on the host platform; emit a binary for the target.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /coffeeshop

# Cache module downloads separately from source build.
COPY go.mod go.sum ./
RUN go mod download

# Copy everything the embed directive needs before building.
COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o depresso-tron-418 .

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM scratch

# Pull in TLS root certs for outbound Gemini API + weather calls.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=build /coffeeshop/depresso-tron-418 /depresso-tron-418

# RFC 2324 compliant port (418 ∪ 80 = 8418 ∪ 80 = 8418, close enough).
ENV PORT=8418
EXPOSE 8418

ENTRYPOINT ["/depresso-tron-418"]
