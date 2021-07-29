FROM golang:alpine AS builder

WORKDIR /restic-robot
ADD . .
ENV GO111MODULE=on
RUN apk add git
RUN apk add gcc
RUN apk add musl-dev
RUN go mod tidy
RUN go build

RUN apk add --no-cache curl && curl -O https://downloads.rclone.org/rclone-current-linux-amd64.zip \
    && unzip rclone-current-linux-amd64.zip \
    && cd rclone-*-linux-amd64 \
    && cp rclone /usr/bin/

FROM restic/restic AS runner

COPY --from=builder /usr/bin/rclone /usr/bin/rclone
COPY --from=builder /restic-robot/restic-robot /usr/bin/restic-robot

ENTRYPOINT ["restic-robot"]
