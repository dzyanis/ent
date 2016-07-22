#!/bin/sh

APP_DIR="$(cd "$(dirname $0)"/.. && pwd)"

POLICY_DIR="${APP_DIR}/config/policies"
DATA_DIR="${ENT_VOLUME:-${APP_DIR}/data}"
OWNER="${ENT_OWNER:-Unknown Owner}"
EMAIL="${ENT_OWNER_EMAIL:-unknown@example.com}"
PORT="${PORT:-8080}"

mkdir -p "${POLICY_DIR}"

for BUCKET in ${ENT_BUCKETS}
do
  cat > "${POLICY_DIR}/${bucket}.entpolicy" <<EOF
{
  "name":"${BUCKET}",
  "owner": {
    "email": {
      "name": "${OWNER}",
      "address": "${EMAIL}"
    }
  }
}
EOF
done

exec "${APP_DIR}/app/ent" \
  -fs.root="${DATA_DIR}" \
  -http.addr=":${PORT}" \
  -provider.dir="${POLICY_DIR}"
