# Chirpy cURL Examples

This file is a credential-safe companion to `requests.txt`.
It mirrors the grader-style request flow, but replaces real values with placeholders.

## Before you start

Use local shell variables so you never hard-code secrets into the file:

```bash
export WALTER_ACCESS_TOKEN="<paste-walter-token-here>"
export SAUL_ACCESS_TOKEN="<paste-saul-token-here>"
export JWT_TOKEN="<paste-jwt-token-here>"
export CHIRP_ID="<paste-chirp-id-here>"
```

## How to add headers

### Bearer token

Use this for authenticated user endpoints:

```bash
-H "Authorization: Bearer $WALTER_ACCESS_TOKEN"
```

or:

```bash
-H "Authorization: Bearer $JWT_TOKEN"
```

### Generic header format

If you need to add any header, the pattern is:

```bash
-H "Header-Name: Header-Value"
```

For example, the webhook endpoint uses an API-key style header:

```bash
-H "Authorization: ApiKey <your-secret-here>"
```

> Keep real tokens in your local shell or `.env`; do not commit them.

## Example flow 

### 1) Reset local state

```bash
curl -i -X POST http://localhost:8080/admin/reset
```

Expected status code: `200`

### 2) Create user Walter

```bash
curl -i -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{
    "email": "walt@breakingbad.com",
    "password": "123456"
  }'
```

Expected status code: `201`

Expected JSON:

- `.email` = `walt@breakingbad.com`

### 3) Login as Walter and capture the secret authentication token

```bash
curl -i -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "walt@breakingbad.com",
    "password": "123456"
  }'
```

Expected status code: `200`

Parse the token from the response body:

```bash
WALTER_ACCESS_TOKEN="$(curl -s -X POST http://localhost:8080/api/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"walt@breakingbad.com","password":"123456"}' | jq -r '.token')"
```

### 4) Create a chirp as Walter

```bash
curl -i -X POST http://localhost:8080/api/chirps \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $WALTER_ACCESS_TOKEN" \
  -d '{
    "body": "I did it for me. I liked it. I was good at it. And I was really... I was alive."
  }'
```

Expected status code: `201`

Parse the chirp ID from the response body:

```bash
CHIRP_ID="$(curl -s -X POST http://localhost:8080/api/chirps \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $WALTER_ACCESS_TOKEN" \
  -d '{"body":"I did it for me. I liked it. I was good at it. And I was really... I was alive."}' | jq -r '.id')"
```

### 5) Fetch Walter's chirp

```bash
curl -i http://localhost:8080/api/chirps/$CHIRP_ID
```

Expected status code: `200`

### 6) Try deleting without a token

```bash
curl -i -X DELETE http://localhost:8080/api/chirps/$CHIRP_ID
```

Expected status code: `401`

### 7) Create Saul

```bash
curl -i -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{
    "email": "saul@bettercall.com",
    "password": "123456"
  }'
```

Expected status code: `201`

Expected JSON:

- `.email` = `saul@bettercall.com`

### 8) Login as user Saul and capture the secret authentication token

```bash
curl -i -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "saul@bettercall.com",
    "password": "123456"
  }'
```

Expected status code: `200`

Parse Saul's access token:

```bash
SAUL_ACCESS_TOKEN="$(curl -s -X POST http://localhost:8080/api/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"saul@bettercall.com","password":"123456"}' | jq -r '.token')"
```

### 9) Try deleting Walter's chirp with Saul's authentication token

```bash
curl -i -X DELETE http://localhost:8080/api/chirps/$CHIRP_ID \
  -H "Authorization: Bearer $SAUL_ACCESS_TOKEN"
```

Expected status code: `403`

### 10) Delete Walter's chirp with Walter's authentication token

```bash
curl -i -X DELETE http://localhost:8080/api/chirps/$CHIRP_ID \
  -H "Authorization: Bearer $WALTER_ACCESS_TOKEN"
```

Expected status code: `204`

### 11) Confirm the chirp is gone

```bash
curl -i http://localhost:8080/api/chirps/$CHIRP_ID
```

Expected status code: `404`

### 12) Refresh Walter's user details

```bash
curl -i -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "walt@breakingbad.com",
    "password": "123456"
  }'
```

Expected status code: `200`

Parse the token into `JWT_TOKEN` if you want to reuse the generic name:

```bash
JWT_TOKEN="$(curl -s -X POST http://localhost:8080/api/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"walt@breakingbad.com","password":"123456"}' | jq -r '.token')"
```

### 13) Update Walter's credentials

```bash
curl -i -X PUT http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{
    "email": "walter@breakingbad.com",
    "password": "losPollosHermanos"
  }'
```

Expected status code: `200`

Expected JSON:

- `.email` = `walter@breakingbad.com`

### 14) Try updating without authentication token

```bash
curl -i -X PUT http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{
    "email": "walter@breakingbad.com",
    "password": "j3ssePinkM@nCantCook"
  }'
```

Expected status code: `401`

### 15) Try updating with a bad authentication token

```bash
curl -i -X PUT http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer badToken" \
  -d '{
    "email": "walter@breakingbad.com",
    "password": "j3ssePinkM@nCantCook"
  }'
```

Expected status code: `401`

## Safety notes

- Do not paste real tokens into the file.
- Keep secrets local in your shell or `.env`.
- If you share this file, keep the placeholder values unchanged.
- The important header formats are:
  - `Authorization: Bearer <token>`
  - `Authorization: ApiKey <secret>`

## Optional webhook example

If you need the webhook flow, use your own local API key value:

```bash
curl -i -X POST http://localhost:8080/api/polka/webhooks \
  -H "Content-Type: application/json" \
  -H "Authorization: ApiKey <your-local-api-key>" \
  -d '{
    "event": "user.upgraded",
    "data": {"user_id": "<user-uuid>"}
  }'
```


