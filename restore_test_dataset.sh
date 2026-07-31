#!/bin/bash

DSN=$(toml get dsn)
migrate -database "${DSN}" -path ./crawler/pkg/crawler/migrations up
psql "${DSN}" -f ./tables_backup.sql
