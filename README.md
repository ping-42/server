- [Ping42 Telemetry Server](#ping42-telemetry-server)
  - [BigQuery Development](#bigquery-development)
  - [The Generator](#the-generator)
  - [Deploying the server](#deploying-the-server)

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

## The Generator

The `generator/` folder contains boilerplate code that dispatches generated events to the function for testing purposes. To run it, try `go run .` in that folder.

## Deploying the server

The server is deployed automatically by the Github Actions. Please see the `.github/workflows` folder for more info.

To deploy by hand:

```bash
gcloud functions deploy dsn-carrier \
    --gen2 \
    --runtime=go121 --region=us-central1 \
    --source=. \
    --entry-point=SignalCarrier \
    --trigger-http \
    --allow-unauthenticated
```
