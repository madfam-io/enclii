-- Templates table for starter templates and marketplace
CREATE TABLE IF NOT EXISTS templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Basic info
    slug VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    long_description TEXT,

    -- Categorization
    category VARCHAR(100) NOT NULL DEFAULT 'starter', -- starter, framework, database, fullstack, api, frontend
    framework VARCHAR(100), -- nextjs, express, fastapi, django, rails, etc.
    language VARCHAR(50), -- typescript, javascript, python, go, ruby, rust
    tags TEXT[], -- Array of tags for filtering

    -- Source
    source_type VARCHAR(50) NOT NULL DEFAULT 'github', -- github, gitlab, internal
    source_repo VARCHAR(500), -- e.g., "vercel/next.js/examples/blog-starter"
    source_branch VARCHAR(100) DEFAULT 'main',
    source_path VARCHAR(500) DEFAULT '/', -- Path within repo if monorepo

    -- Template config (what to create when deployed)
    config JSONB NOT NULL DEFAULT '{}', -- Services, env vars, databases, etc.

    -- Display
    icon_url VARCHAR(500),
    preview_url VARCHAR(500), -- Live demo URL
    screenshot_urls TEXT[], -- Gallery images

    -- Metadata
    author VARCHAR(255),
    author_url VARCHAR(500),
    documentation_url VARCHAR(500),

    -- Stats
    deploy_count INTEGER DEFAULT 0,
    star_count INTEGER DEFAULT 0,

    -- Visibility
    is_official BOOLEAN DEFAULT false, -- Enclii-maintained
    is_featured BOOLEAN DEFAULT false, -- Show on homepage
    is_public BOOLEAN DEFAULT true, -- Public visibility

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for efficient lookups
CREATE INDEX idx_templates_slug ON templates(slug);
CREATE INDEX idx_templates_category ON templates(category);
CREATE INDEX idx_templates_framework ON templates(framework);
CREATE INDEX idx_templates_language ON templates(language);
CREATE INDEX idx_templates_is_featured ON templates(is_featured) WHERE is_featured = true;
CREATE INDEX idx_templates_is_official ON templates(is_official) WHERE is_official = true;
CREATE INDEX idx_templates_deploy_count ON templates(deploy_count DESC);

-- Template deployments (tracks who deployed what template)
CREATE TABLE IF NOT EXISTS template_deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,

    -- Deployment details
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, in_progress, completed, failed
    error_message TEXT,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_template_deployments_template ON template_deployments(template_id);
CREATE INDEX idx_template_deployments_project ON template_deployments(project_id);
CREATE INDEX idx_template_deployments_user ON template_deployments(user_id);

-- Insert official starter templates
INSERT INTO templates (slug, name, description, category, framework, language, tags, source_type, source_repo, source_branch, config, icon_url, is_official, is_featured) VALUES
-- Next.js
('nextjs-starter', 'Next.js Starter', 'A minimal Next.js app with TypeScript and Tailwind CSS', 'starter', 'nextjs', 'typescript', ARRAY['react', 'tailwind', 'typescript'], 'github', 'vercel/next.js', 'canary',
'{"services": [{"name": "web", "type": "web", "build": {"type": "nixpacks"}, "port": 3000, "env_vars": {"NODE_ENV": "production"}}]}',
'https://assets.vercel.com/image/upload/v1607554385/repositories/next-js/next-logo.png', true, true),

-- Express.js
('express-api', 'Express.js API', 'RESTful API with Express.js, TypeScript, and PostgreSQL', 'api', 'express', 'typescript', ARRAY['nodejs', 'rest', 'postgresql'], 'github', 'expressjs/express', 'master',
'{"services": [{"name": "api", "type": "web", "build": {"type": "nixpacks"}, "port": 3000, "env_vars": {"NODE_ENV": "production"}}], "databases": [{"type": "postgres", "name": "db"}]}',
'https://raw.githubusercontent.com/expressjs/expressjs.com/gh-pages/images/express-facebook-share.png', true, true),

-- FastAPI
('fastapi-starter', 'FastAPI Starter', 'Modern Python API with FastAPI, async support, and OpenAPI docs', 'api', 'fastapi', 'python', ARRAY['python', 'async', 'openapi'], 'github', 'tiangolo/fastapi', 'master',
'{"services": [{"name": "api", "type": "web", "build": {"type": "nixpacks"}, "port": 8000, "env_vars": {"PYTHONUNBUFFERED": "1"}}]}',
'https://fastapi.tiangolo.com/img/logo-margin/logo-teal.png', true, true),

-- Django
('django-starter', 'Django Starter', 'Full-stack Django app with PostgreSQL and admin panel', 'fullstack', 'django', 'python', ARRAY['python', 'postgresql', 'admin'], 'github', 'django/django', 'main',
'{"services": [{"name": "web", "type": "web", "build": {"type": "nixpacks"}, "port": 8000, "env_vars": {"DJANGO_SETTINGS_MODULE": "config.settings.production"}}], "databases": [{"type": "postgres", "name": "db"}]}',
'https://static.djangoproject.com/img/logos/django-logo-positive.png', true, true),

-- Go Fiber
('go-fiber-api', 'Go Fiber API', 'High-performance Go API with Fiber framework', 'api', 'fiber', 'go', ARRAY['go', 'fiber', 'rest'], 'github', 'gofiber/fiber', 'master',
'{"services": [{"name": "api", "type": "web", "build": {"type": "dockerfile"}, "port": 3000, "env_vars": {"GIN_MODE": "release"}}]}',
'https://gofiber.io/assets/images/logo.svg', true, true),

-- React + Vite
('react-vite', 'React + Vite', 'Fast React app with Vite, TypeScript, and TailwindCSS', 'frontend', 'react', 'typescript', ARRAY['react', 'vite', 'tailwind'], 'github', 'vitejs/vite', 'main',
'{"services": [{"name": "web", "type": "static", "build": {"type": "nixpacks", "output_dir": "dist"}, "port": 80}]}',
'https://vitejs.dev/logo.svg', true, true),

-- Astro
('astro-blog', 'Astro Blog', 'Content-focused blog with Astro and Markdown support', 'frontend', 'astro', 'typescript', ARRAY['astro', 'blog', 'markdown', 'ssg'], 'github', 'withastro/astro', 'main',
'{"services": [{"name": "web", "type": "static", "build": {"type": "nixpacks", "output_dir": "dist"}, "port": 80}]}',
'https://astro.build/assets/press/astro-icon-light.svg', true, false),

-- Flask
('flask-api', 'Flask API', 'Lightweight Python API with Flask and SQLAlchemy', 'api', 'flask', 'python', ARRAY['python', 'flask', 'sqlalchemy'], 'github', 'pallets/flask', 'main',
'{"services": [{"name": "api", "type": "web", "build": {"type": "nixpacks"}, "port": 5000, "env_vars": {"FLASK_ENV": "production"}}]}',
'https://flask.palletsprojects.com/en/2.0.x/_images/flask-logo.png', true, false),

-- NestJS
('nestjs-api', 'NestJS API', 'Enterprise-grade Node.js API with NestJS and TypeORM', 'api', 'nestjs', 'typescript', ARRAY['nodejs', 'nestjs', 'typeorm', 'postgresql'], 'github', 'nestjs/nest', 'master',
'{"services": [{"name": "api", "type": "web", "build": {"type": "nixpacks"}, "port": 3000, "env_vars": {"NODE_ENV": "production"}}], "databases": [{"type": "postgres", "name": "db"}]}',
'https://nestjs.com/img/logo-small.svg', true, false),

-- SvelteKit
('sveltekit-starter', 'SvelteKit Starter', 'Full-stack SvelteKit app with SSR and TypeScript', 'fullstack', 'sveltekit', 'typescript', ARRAY['svelte', 'sveltekit', 'ssr'], 'github', 'sveltejs/kit', 'main',
'{"services": [{"name": "web", "type": "web", "build": {"type": "nixpacks"}, "port": 3000, "env_vars": {"NODE_ENV": "production"}}]}',
'https://svelte.dev/svelte-logo-horizontal.svg', true, false);

-- Function to update deploy count
CREATE OR REPLACE FUNCTION increment_template_deploy_count()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'completed' AND (OLD IS NULL OR OLD.status != 'completed') THEN
        UPDATE templates SET deploy_count = deploy_count + 1, updated_at = NOW()
        WHERE id = NEW.template_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_increment_deploy_count
AFTER INSERT OR UPDATE ON template_deployments
FOR EACH ROW EXECUTE FUNCTION increment_template_deploy_count();
