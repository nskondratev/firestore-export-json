# firestore-export-json
CLI app for exporting the GCP Firestore database in json format.
The app allows you to download all data from the Firestore database in json format, including all subcollections, and prints it to stdout.
Note: all data is stored in memory before being converted to json, so this app is not suitable for exporting large databases.

## Prerequisites
* Installed Go version 1.18 or higher. [Installation instructions](https://go.dev/doc/install).

## Usage
* Build the app: `go build -o firestore-export .`.
* Export data to file: `./firestore-export -project yourgcpproject -credentials /path/to/json/with/creds/to/gcp.json > data.json`.

Available options:
* `-project` - GCP project ID.
* `-credentials` - path to json file with service account credentials.
* `-concurrency` - limit on the number of launched goroutines (default = 1000).
