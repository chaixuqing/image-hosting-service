# image-hosting-service

Minimal HTTP image hosting service.

## Run

```bash
go run .
```

Optional environment variables:

- `PORT` (default `8080`)
- `IMAGE_STORAGE_DIR` (default `uploads`)

## API

- `POST /images` (multipart field name: `image`)
- `GET /images/{id}`
- `DELETE /images/{id}`
- `GET /healthz`

### Example upload

```bash
curl -F "image=@/path/to/image.png" http://localhost:8080/images
```

## Test

```bash
go test ./...
```
