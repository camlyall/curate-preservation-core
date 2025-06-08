
# Penwern Preservation Core

## Execution

- Validate Inputs and Endpoints
- ~~Get Node and Child Node Data (Cells API)~~
- ~~Configure CEC - Not Required~~
- ~~Download Package (cec + token)~~
- ~~Gather Metadata (Cells API + token)~~
- ~~Preprocessing~~
  - ~~Constuct Transfer Package~~
  - ~~Generate Metadata Files~~
- ~~Submit to A3M~~
- ~~Retrieve AIP~~
- ~~Validate AIP Existence~~
- Postprocessing
  - ~~Extract AIP~~
    - If not in desired format
  - ~~Compress AIP~~
    - Into desired format
- ~~Upload DIP to AtoM - if required~~
- ~~Upload AIP to Pydio Cells~~
- ~~Remove the processing directory~~

## Requirements

- Penwern A3M (A4M)
  - Shared file system required
- Cells Enterprise Client Binary (CEC)
- Pydio Cells
  - Preservation Metadata Namespace - `usermeta-preservation-status` (required)
  - Dissemination Metadata Namespace - `usermeta-dip-status` (optional)
  - AtoM Sluf Metadata Namespace - `usermeta-atom-slug` (optional)

***Note***: Metadata namespaces must be editable by the user. Admin user doesn't have access to users personal files, so, cannot edit the tag.

***Note***: `usermeta-preservation-status` is new and depreciation of `usermeta-a3m-progress` is on-going and still currently supported.

***Note***: `usermeta-atom-slug` is new and depreciation of `usermeta-atom-linked-description` is on-going and still currently supported.

## System Requirements

```bash
# For XML Schema Validation
sudo apt install libxml2
```

## ProtoBuf

Buf is a tool for generating code from Protocol Buffers definitions.

- A3M Protos: <https://buf.build/penwern/a3m>
- Buf: <https://buf.build/docs/installation>

### Generating Code

Generate the a3m go code from the protos:

```bash
buf generate
```

## Development

Make volumes

```bash
mkdir -p /tmp/preservation/a3m_completed /tmp/preservation/a3m_dips
```

Build development environment

```bash
docker compose up -d
```

Re-build preservation

```bash
docker compose build preservation
```

Re-build running container

```bash
docker compose up preservation --build -d
```

Execute in command line

```bash
# Assuming admin user exists with test-dir/ in personal-files/
go run . -u admin -p personal-files/test-dir
```

Execute in request

```bash
# Send a POST request to the preservation endpoint
curl -X POST http://localhost:6905/preserve \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "paths": ["personal-files/test-dir"]
  }'
```
