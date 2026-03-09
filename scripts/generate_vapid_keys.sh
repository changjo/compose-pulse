#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
ENV_FILE="${1:-$ROOT_DIR/.env}"

if [ ! -f "$ENV_FILE" ]; then
  echo "env file not found: $ENV_FILE" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker command not found; cannot generate VAPID keys" >&2
  exit 1
fi

KEY_OUTPUT="$(docker run --rm golang:1.22-alpine sh -lc 'cat >/tmp/gen_vapid.go <<\"EOF\"
package main

import (
  \"crypto/ecdsa\"
  \"crypto/elliptic\"
  \"crypto/rand\"
  \"encoding/base64\"
  \"fmt\"
)

func pad32(in []byte) []byte {
  out := make([]byte, 32)
  if len(in) >= 32 {
    copy(out, in[len(in)-32:])
    return out
  }
  copy(out[32-len(in):], in)
  return out
}

func main() {
  key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
  if err != nil {
    panic(err)
  }

  x := pad32(key.PublicKey.X.Bytes())
  y := pad32(key.PublicKey.Y.Bytes())
  pub := make([]byte, 65)
  pub[0] = 0x04
  copy(pub[1:33], x)
  copy(pub[33:65], y)

  priv := pad32(key.D.Bytes())
  enc := base64.RawURLEncoding

  fmt.Printf(\"WEB_PUSH_VAPID_PUBLIC_KEY=%s\\n\", enc.EncodeToString(pub))
  fmt.Printf(\"WEB_PUSH_VAPID_PRIVATE_KEY=%s\\n\", enc.EncodeToString(priv))
}
EOF
go run /tmp/gen_vapid.go')"

PUBLIC_KEY=$(printf "%s\n" "$KEY_OUTPUT" | awk -F= '/^WEB_PUSH_VAPID_PUBLIC_KEY=/{print $2}')
PRIVATE_KEY=$(printf "%s\n" "$KEY_OUTPUT" | awk -F= '/^WEB_PUSH_VAPID_PRIVATE_KEY=/{print $2}')

if [ -z "$PUBLIC_KEY" ] || [ -z "$PRIVATE_KEY" ]; then
  echo "failed to generate VAPID keys" >&2
  exit 1
fi

TMP_FILE="${ENV_FILE}.tmp.$$"
trap 'rm -f "$TMP_FILE"' EXIT

awk -v pub="$PUBLIC_KEY" -v priv="$PRIVATE_KEY" '
BEGIN {
  updated_pub = 0
  updated_priv = 0
}
/^[[:space:]]*WEB_PUSH_VAPID_PUBLIC_KEY=/ {
  print "WEB_PUSH_VAPID_PUBLIC_KEY=" pub
  updated_pub = 1
  next
}
/^[[:space:]]*WEB_PUSH_VAPID_PRIVATE_KEY=/ {
  print "WEB_PUSH_VAPID_PRIVATE_KEY=" priv
  updated_priv = 1
  next
}
{ print }
END {
  if (updated_pub == 0) {
    print "WEB_PUSH_VAPID_PUBLIC_KEY=" pub
  }
  if (updated_priv == 0) {
    print "WEB_PUSH_VAPID_PRIVATE_KEY=" priv
  }
}
' "$ENV_FILE" >"$TMP_FILE"

mv "$TMP_FILE" "$ENV_FILE"
trap - EXIT

echo "updated WEB_PUSH_VAPID_PUBLIC_KEY and WEB_PUSH_VAPID_PRIVATE_KEY in $ENV_FILE"
