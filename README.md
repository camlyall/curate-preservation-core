
Requires running A3M Server with a shared file system for transfers and AIP Retrieval.

**NOTE**: Metadata Tag usermeta-a3m-progress needs to be editable by the user. Admin user doesn't have access to users personal files, therefore cannot edit the tag.

# Process flow
- Requirements:
  - A3M Server
    - Shared File System
  - Pydio Cells Server
  - AtoM (Optional)
- Input Parameters:
  - Pydio Cells
    - Endpoint (e.g. https://curate.penwern.co.uk)
    - Token
    - Package Path
  - A3M
    - GRPC Endpoint (e.g. http://localhost:7000)
    - Processing Configuration
  - AtoM
    - Endpoint (e.g. https://atom.penwern.com)
    - Authentication
      - Username & Password
      - or
      - API Key
  - Processing
    - Processing Configuration (e.g. ZIP AIP)
- Execution
  - Validate Inputs and Endpoints
  - ~~Get Node and Child Node Data (Cells API)~~
  - ~~Configure CEC - Not Required~~
  - ~~Download Package (cec + token)~~
  - Gather Metadata (Cells API + token)
  - Preprocessing
    - ~~Constuct Transfer Package~~
    - Generate Metadata Files
  - ~~Submit to A3M~~
  - ~~Retrieve AIP~~
  - Validate AIP
  - Postprocessing
    - ~~Extract AIP~~
      - If not in desired format
    - ~~Compress AIP~~
      - Into desired format
  - Upload DIP to AtoM - if required
  - ~~Upload AIP to Pydio Cells~~
  - ~~Remove the processing directory~~

# Requirements
```bash
# For XML Schema Validation
sudo apt-get install libxml2
```

# ProtoBuf
Buf is a tool for generating code from Protocol Buffers definitions.
- A3M Protos: https://buf.build/artefactual/a3m
- Buf: https://buf.build/docs/installation


# Generating Code
Generate the a3m go code from the protos:
```bash
buf generate
```

# Development

Build A3M Server
```bash
docker compose up -d
```

Cells package path is hardcoded in the demo to `personal-files/test_dir`. Must create the directory before running the demo. 

Run demo
```bash
go run cmd/main.go
```
