# MADFAM Private npm Registry (npm.madfam.io)

Private npm registry for MADFAM ecosystem packages using Verdaccio.

## Quick Start

```bash
# 1. Start the registry
docker-compose up -d

# 2. Create initial admin user
docker exec -it npm-madfam-org npx verdaccio-htpasswd -b /verdaccio/conf/htpasswd admin <password>

# 3. Configure npm/pnpm to use registry
npm config set @madfam:registry https://npm.madfam.io
npm config set @janua:registry https://npm.madfam.io
npm config set @cotiza:registry https://npm.madfam.io
npm config set @fortuna:registry https://npm.madfam.io
npm config set @avala:registry https://npm.madfam.io
npm config set @forgesight:registry https://npm.madfam.io
npm config set @coforma:registry https://npm.madfam.io
npm config set @forj:registry https://npm.madfam.io
npm config set @enclii:registry https://npm.madfam.io

# 4. Login to registry
npm login --registry https://npm.madfam.io
```

## Publishing Packages

```bash
# From any MADFAM package directory
npm publish --registry https://npm.madfam.io
```

## Registered Scopes

| Scope | Packages |
|-------|----------|
| `@madfam/*` | ui, analytics, tsconfig, geom-core |
| `@janua/*` | react-sdk, node-sdk |
| `@cotiza/*` | client, shared, ui, pricing-engine |
| `@fortuna/*` | client |
| `@avala/*` | client |
| `@forgesight/*` | client |
| `@coforma/*` | client, types, ui |
| `@forj/*` | client |
| `@enclii/*` | cli, sdk |

## CI/CD Integration

Add to GitHub Actions:

```yaml
- name: Setup npm registry
  run: |
    echo "@madfam:registry=https://npm.madfam.io" >> ~/.npmrc
    echo "//npm.madfam.io/:_authToken=\${{ secrets.NPM_MADFAM_TOKEN }}" >> ~/.npmrc
```

## Local Development

For local development, packages can still use `file:` references.
The registry is for production deployments (Docker/Enclii).

## Backup

Storage is persisted in Docker volume `verdaccio-storage`.
Backup regularly:

```bash
docker run --rm -v verdaccio-storage:/data -v $(pwd):/backup alpine tar czf /backup/verdaccio-backup.tar.gz /data
```
