# Restic Robot

Backups done right... by robots with rclone !

This is a small and simple wrapper application for [Restic](https://github.com/restic/restic/) that provides:

- Automatically creates repository if it doesn't already exist
- Scheduled backups - no need for system-wide cron
- Prometheus metrics- know when your backups don't run!
- JSON logs - for the robots!
- Pre/post shell script hooks for custom behaviour! (Thanks @opthomas-prime!)

## Usage

Just `go build` and run it, or, if you're into Docker, `thomaslacaze/restic-robot-rclone`.

Environment variables:

- `SCHEDULE`: cron schedule
- `RESTIC_REPOSITORY`: repository name
- `RESTIC_PASSWORD`: repository password
- `RESTIC_ARGS`: additional args for backup command
- `RUN_ON_BOOT`: run a backup on startup
- `PROMETHEUS_ENDPOINT`: metrics endpoint
- `PROMETHEUS_ADDRESS`: metrics host:port
- `PRE_COMMAND`: A shell command to run before a backup starts
- `POST_COMMAND`: A shell command to run if the backup completes successfully

Prometheus metrics:

- `backups_all_total`: The total number of backups attempted, including failures.
- `backups_successful_total`: The total number of backups that succeeded.
- `backups_failed_total`: The total number of backups that failed.
- `backup_duration_milliseconds`: The duration of backups in milliseconds.
- `backup_files_new`: Amount of new files.
- `backup_files_changed`: Amount of files with changes.
- `backup_files_unmodified`: Amount of files unmodified since last backup.
- `backup_files_processed`: Total number of files scanned by the backup for changes.
- `backup_added_bytes`: Total number of bytes added to the repository.
- `backup_processed_bytes`: Total number of bytes scanned by the backup for changes

It's that simple!

## Docker Compose

Stick this in with your other compose services for instant backups!

```yml
services:
  #
  # your stuff etc...
  #

  backup:
    image: thomaslacaze/restic-robot-rclone
    restart: always
    hostname: ${YOUR_HOSTNAME}
    environment:
      # every day at 2am
      SCHEDULE: 0 0 2 * * *
      RESTIC_REPOSITORY: my_service_repository #example rclone:s3:Restic
      RESTIC_PASSWORD: ${MY_SERVICE_RESTIC_PASSWORD}
      # restic-robot runs `restic backup ${RESTIC_ARGS}`
      # so this is where you specify the directory and any other args.
      RESTIC_ARGS: backup /data  --exclude="/data/go/*"
      B2_ACCOUNT_ID: ${B2_ACCOUNT_ID}
      B2_ACCOUNT_KEY: ${B2_ACCOUNT_KEY}
      PROMETHEUS_ADDRESS: 0.0.0.0:9819
    volumes:
      # Bind whatever directories to the backup container.
      # You can safely bind the same directory to multiple containers.
      - "/container_data/blog/wordpress:/data/wordpress"
```
