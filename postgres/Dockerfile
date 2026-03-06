FROM postgres:16-alpine
# Bake all init/migration SQL into the image so no volume mount is needed.
# PostgreSQL runs scripts in /docker-entrypoint-initdb.d/ alphabetically on first start.
COPY postgres/init/ /docker-entrypoint-initdb.d/
