- [Ping42 Telemetry Server](#ping42-telemetry-server)
  - [BigQuery Development](#bigquery-development)
  - [CLI](#cli)

# Ping42 Telemetry Server

To run the local dev server:

```bash
cd server
go run .Flags
```

## BigQuery Development

To clean up the BQ datasets, one can has to delete the functions that are using it first:

```bash
gcloud functions list
gcloud functions delete server
```

Then figure out which datasets to delete.

```bash
gcloud alpha bq datasets list
gcloud alpha bq datasets delete clients --remove-tables
```

> Note: In some cases a web based session could be causing the above to fail. Tough luck - delete the datasets from the BigQuery part of the GCP console.


## CLI

Run migrations:

```bash
 go run . run -p 8080
```

Create new sersor:

```bash
 go run . mksensor -n SensorName -l SensorLocaltion
```

Run migrations:

```bash
 go run . migrate
```