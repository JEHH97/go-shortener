# Deployment notes

The included `render.yaml` is a reference blueprint, not a deployment action. Before using a hosting provider, provision a private Redis instance and enter its endpoint and password, set a strong unique API key, and replace the example public base URL. Configure TLS at the provider edge and restrict access to observability endpoints as appropriate. Do not commit generated credentials. Database schema setup runs automatically when the app starts.

The service has no deploy workflow. Production deployment, domain configuration, backups, retention policies, and secrets management remain operator-owned.
