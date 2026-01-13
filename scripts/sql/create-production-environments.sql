-- create-production-environments.sql
-- Fixes the "environment not found" error for auto-deploy
--
-- Run via: kubectl exec -n enclii deploy/postgres -- psql -U postgres -d enclii -f /path/to/this.sql
-- Or:      psql -h <host> -U <user> -d enclii -f create-production-environments.sql

-- Step 1: Check current state
\echo '=== Current Projects ==='
SELECT id, name, slug, created_at FROM projects ORDER BY created_at;

\echo '=== Current Environments ==='
SELECT e.id, p.slug as project, e.name, e.kube_namespace, e.created_at
FROM environments e
JOIN projects p ON e.project_id = p.id
ORDER BY p.slug, e.name;

\echo '=== Services with auto_deploy enabled ==='
SELECT s.id, s.name, p.slug as project, s.auto_deploy, s.git_repo
FROM services s
JOIN projects p ON s.project_id = p.id
WHERE s.auto_deploy = true;

\echo '=== Projects WITHOUT production environment ==='
SELECT p.id, p.slug, p.name
FROM projects p
WHERE NOT EXISTS (
    SELECT 1 FROM environments e
    WHERE e.project_id = p.id AND e.name = 'production'
);

-- Step 2: Create production environments for all projects that don't have one
\echo '=== Creating production environments ==='
INSERT INTO environments (id, project_id, name, kube_namespace, created_at, updated_at)
SELECT
    gen_random_uuid(),
    p.id,
    'production',
    'enclii',  -- All production services use the 'enclii' namespace
    NOW(),
    NOW()
FROM projects p
WHERE NOT EXISTS (
    SELECT 1 FROM environments e
    WHERE e.project_id = p.id AND e.name = 'production'
);

-- Step 3: Verify creation
\echo '=== Verification: All production environments ==='
SELECT e.id, p.slug as project, e.name, e.kube_namespace, e.created_at
FROM environments e
JOIN projects p ON e.project_id = p.id
WHERE e.name = 'production'
ORDER BY p.slug;

-- Step 4: Check pending releases that can now be deployed
\echo '=== Pending releases ready for deployment ==='
SELECT
    r.id as release_id,
    s.name as service,
    r.version,
    r.image_uri,
    r.status,
    r.created_at
FROM releases r
JOIN services s ON r.service_id = s.id
WHERE r.status IN ('pending', 'ready')
ORDER BY r.created_at DESC
LIMIT 10;

\echo '=== DONE: Production environments created ==='
\echo 'Auto-deploy should now work. Trigger a build via git push or manually.'
