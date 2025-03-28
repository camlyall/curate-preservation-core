
Requires running A3M Server with a shared file system for transfers and AIP Retrieval.

**TODO**: 
- A3M Failed jobs
  - Premis failure
- Premis and metadata already in metadata folder??

**NOTE**: Metadata Tag usermeta-a3m-progress needs to be editable by the user. Admin user doesn't have access to users personal files, therefore cannot edit the tag.

# Notes
- If `usermeta-a3m-progress` exists on the node, we use that for preservation progress. Otherwise, we use the new `usermeta-preservation-status`, which must exist and be editable by the user.

# Requirements:
- A3M Server
- Cells Enterprise Client (CEC)
- Pydio Cells
  - Metadata Namespace `usermeta-preservation-status`

# Process flow
- Requirements:
  - A3M Server
  - Pydio Cells Server
  - AtoM (Optional)
- Environment:
  - Processing directory
  - Pydio Cells
    - CEC binary path
    - Cells Address
    - Cells Archive Directory
  -   Cells Admin Token
  - A3M
    - A3M Address
    - A3M Completed Directory
- Input Parameters:
  - Cells User
  - Cells Package Path
  - Clean Up (Optional)
  - Cells Archive Directory (Overwrites environment var)

- Undecided:
  - Processing Configuration
  - AtoM
    - Endpoint (e.g. https://atom.penwern.com)
    - Authentication
      - Username & Password
      - or
      - API Key

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

Build development environment
```bash
# Starts required containers for pydio cells (+mysql), a3m and preservation endpoint
docker compose up -d
```

Re-build preservation
```bash
docker compose build preservation
```

Execute in command line
```bash
# Assuming test user exists with test_dir/ in personal-files/
go run . -u test -p personal-files/test_dir
```

Execute in request
```bash
# Send a POST request to the preservation endpoint
curl -X POST http://localhost:6905/preserve \
    -H "Content-Type: application/json" \
    -d '{
      "username": "test",
      "paths": ["personal-files/test_dir"],
      "cleanup": true
    }'
```
