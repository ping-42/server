# PING42 Telemetry Server

This service receives telemetry connections from the sensor network and diligently stores its telemetry in a database.

> Note: This service is intended to be ran during either development or on the production infrastructure.

# Development

To run the local dev server:

```bash
go run . --help # for more info
```

## CLI Arguments

Run the actual server:

```bash
 go run . run
```

Create new sersor:

```bash
 go run . mksensor -n SensorName -l SensorLocaltion
```

Run migrations:

```bash
 go run . migrate
```