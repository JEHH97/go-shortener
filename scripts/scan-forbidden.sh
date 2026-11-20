#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if grep -RniE --exclude-dir=.git --exclude=scan-forbidden.sh \
  'site''doc|at''las|sky''line|le''ns|ops''center|one''web|eute''lsat|be''am|git\.via''sat\.com|@via''sat\.com|cé''dula|(^|[^0-9])(\+?[0-9]{1,3}[ -])?\(?[0-9]{3}\)?[ -][0-9]{3}[ -][0-9]{4}([^0-9]|$)' .; then
  echo "Forbidden content found." >&2
  exit 1
fi
echo "Forbidden-term scan passed."
