# M7 Failure Drills

Run against a local or staging gateway:

```bash
GATEWAY_URL=http://127.0.0.1:8080 API_KEY=tg-local-dev-key tests/failure/drills.sh
```

The script checks health, auth rejection, provider/route behavior, and the M7
metrics contract. Redis, DB, provider, settlement, callback, snapshot, and
worker restart drills should be executed by changing the target environment and
then re-running the same script plus the load test.
