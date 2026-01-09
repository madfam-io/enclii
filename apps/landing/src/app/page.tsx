import { ArrowRight, Zap, RefreshCw, BarChart3, GitBranch, Check, ExternalLink } from 'lucide-react'

export default function Home() {
  return (
    <main className="min-h-screen">
      {/* Navigation */}
      <nav className="fixed top-0 left-0 right-0 z-50 bg-white/80 dark:bg-gray-900/80 backdrop-blur-md border-b border-gray-200 dark:border-gray-800">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between items-center h-16">
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 bg-primary-600 rounded-lg flex items-center justify-center">
                <span className="text-white font-bold text-lg">E</span>
              </div>
              <span className="font-bold text-xl text-gray-900 dark:text-white">Enclii</span>
            </div>
            <div className="flex items-center gap-4">
              <a
                href="https://docs.enclii.dev"
                className="text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white transition-colors"
              >
                Docs
              </a>
              <a
                href="https://app.enclii.dev"
                className="inline-flex items-center gap-2 bg-primary-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-primary-700 transition-colors"
              >
                Get Started
                <ArrowRight className="w-4 h-4" />
              </a>
            </div>
          </div>
        </div>
      </nav>

      {/* Hero Section */}
      <section className="hero-gradient pt-32 pb-20 px-4 sm:px-6 lg:px-8">
        <div className="max-w-4xl mx-auto text-center">
          <div className="inline-flex items-center gap-2 bg-white/10 backdrop-blur-sm px-4 py-2 rounded-full text-white/90 text-sm mb-8">
            <span className="inline-block w-2 h-2 bg-green-400 rounded-full animate-pulse"></span>
            Production Ready
          </div>
          <h1 className="text-4xl sm:text-5xl lg:text-6xl font-bold text-white mb-6 leading-tight">
            Deploy Without<br />the Bill Shock
          </h1>
          <p className="text-xl text-white/80 mb-10 max-w-2xl mx-auto">
            Railway-style PaaS at 95% less cost. Auto-scaling, zero-downtime deployments,
            and built-in observability on cost-effective infrastructure.
          </p>
          <div className="flex flex-col sm:flex-row gap-4 justify-center">
            <a
              href="https://app.enclii.dev"
              className="inline-flex items-center justify-center gap-2 bg-white text-primary-700 px-8 py-4 rounded-xl font-semibold text-lg hover:bg-gray-100 transition-colors shadow-lg"
            >
              Start Deploying
              <ArrowRight className="w-5 h-5" />
            </a>
            <a
              href="https://docs.enclii.dev"
              className="inline-flex items-center justify-center gap-2 bg-white/10 text-white border border-white/20 px-8 py-4 rounded-xl font-semibold text-lg hover:bg-white/20 transition-colors"
            >
              View Documentation
              <ExternalLink className="w-5 h-5" />
            </a>
          </div>
        </div>
      </section>

      {/* Features Section */}
      <section className="py-24 px-4 sm:px-6 lg:px-8 bg-gray-50 dark:bg-gray-900">
        <div className="max-w-7xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white mb-4">
              Everything You Need to Ship Fast
            </h2>
            <p className="text-lg text-gray-600 dark:text-gray-400 max-w-2xl mx-auto">
              Enterprise-grade deployment infrastructure without the enterprise price tag.
            </p>
          </div>
          <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-8">
            <FeatureCard
              icon={<Zap className="w-8 h-8" />}
              title="Auto-Scaling"
              description="Scale to zero when idle, scale to millions under load. Pay only for what you use."
            />
            <FeatureCard
              icon={<RefreshCw className="w-8 h-8" />}
              title="Zero-Downtime Deploys"
              description="Canary and blue-green deployment strategies with automatic rollback on failure."
            />
            <FeatureCard
              icon={<BarChart3 className="w-8 h-8" />}
              title="Built-in Observability"
              description="Logs, metrics, and traces included. Know exactly what your services are doing."
            />
            <FeatureCard
              icon={<GitBranch className="w-8 h-8" />}
              title="Git-Connected CI/CD"
              description="Push to deploy. Preview environments for every PR. Production from main."
            />
          </div>
        </div>
      </section>

      {/* Pricing Comparison */}
      <section className="py-24 px-4 sm:px-6 lg:px-8">
        <div className="max-w-4xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white mb-4">
              Real Savings, Real Infrastructure
            </h2>
            <p className="text-lg text-gray-600 dark:text-gray-400">
              Same capabilities. Fraction of the cost.
            </p>
          </div>
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl overflow-hidden border border-gray-200 dark:border-gray-700">
            <div className="grid md:grid-cols-2 divide-y md:divide-y-0 md:divide-x divide-gray-200 dark:divide-gray-700">
              {/* Traditional Stack */}
              <div className="p-8">
                <h3 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">Traditional SaaS Stack</h3>
                <div className="space-y-4">
                  <div className="flex justify-between">
                    <span className="text-gray-600 dark:text-gray-400">Railway Pro</span>
                    <span className="font-medium text-gray-900 dark:text-white">$2,000/mo</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-600 dark:text-gray-400">Auth0 B2B Essentials</span>
                    <span className="font-medium text-gray-900 dark:text-white">$220/mo</span>
                  </div>
                  <div className="h-px bg-gray-200 dark:bg-gray-700 my-4"></div>
                  <div className="flex justify-between">
                    <span className="font-semibold text-gray-900 dark:text-white">Monthly Total</span>
                    <span className="font-bold text-2xl text-red-600">$2,220</span>
                  </div>
                </div>
              </div>
              {/* Enclii Stack */}
              <div className="p-8 bg-primary-50 dark:bg-primary-900/20">
                <div className="flex items-center gap-2 mb-4">
                  <h3 className="text-xl font-semibold text-gray-900 dark:text-white">Enclii Stack</h3>
                  <span className="bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100 text-xs font-medium px-2 py-1 rounded-full">
                    95% Less
                  </span>
                </div>
                <div className="space-y-4">
                  <div className="flex justify-between">
                    <span className="text-gray-600 dark:text-gray-400">Hetzner + k3s</span>
                    <span className="font-medium text-gray-900 dark:text-white">$45/mo</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-600 dark:text-gray-400">Ubicloud PostgreSQL</span>
                    <span className="font-medium text-gray-900 dark:text-white">$50/mo</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-600 dark:text-gray-400">Cloudflare (R2, Tunnel)</span>
                    <span className="font-medium text-gray-900 dark:text-white">$5/mo</span>
                  </div>
                  <div className="h-px bg-gray-200 dark:bg-gray-700 my-4"></div>
                  <div className="flex justify-between">
                    <span className="font-semibold text-gray-900 dark:text-white">Monthly Total</span>
                    <span className="font-bold text-2xl text-green-600">~$100</span>
                  </div>
                </div>
              </div>
            </div>
            {/* Savings Banner */}
            <div className="bg-primary-600 p-6 text-center">
              <p className="text-white text-lg">
                <span className="font-bold">5-year savings: $127,200</span>
                <span className="text-white/80 ml-2">with zero vendor lock-in</span>
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Why Enclii Section */}
      <section className="py-24 px-4 sm:px-6 lg:px-8 bg-gray-50 dark:bg-gray-900">
        <div className="max-w-4xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white mb-4">
              Built for Teams Who Ship
            </h2>
          </div>
          <div className="space-y-6">
            <BenefitRow text="Deploy any Dockerfile or use auto-detection with Nixpacks/Buildpacks" />
            <BenefitRow text="Automatic SSL certificates and custom domain routing" />
            <BenefitRow text="Preview environments for every pull request" />
            <BenefitRow text="Built-in secrets management with rotation support" />
            <BenefitRow text="Cost tracking with budget alerts before you overspend" />
            <BenefitRow text="Open source with AGPL-3.0 license - no vendor lock-in" />
          </div>
        </div>
      </section>

      {/* CTA Section */}
      <section className="py-24 px-4 sm:px-6 lg:px-8">
        <div className="max-w-4xl mx-auto text-center">
          <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white mb-6">
            Ready to Deploy Smarter?
          </h2>
          <p className="text-lg text-gray-600 dark:text-gray-400 mb-10 max-w-2xl mx-auto">
            Join teams who are shipping faster while keeping costs under control.
          </p>
          <a
            href="https://app.enclii.dev"
            className="inline-flex items-center gap-2 bg-primary-600 text-white px-8 py-4 rounded-xl font-semibold text-lg hover:bg-primary-700 transition-colors shadow-lg"
          >
            Get Started Free
            <ArrowRight className="w-5 h-5" />
          </a>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-gray-200 dark:border-gray-800 py-12 px-4 sm:px-6 lg:px-8">
        <div className="max-w-7xl mx-auto">
          <div className="grid md:grid-cols-4 gap-8">
            <div>
              <div className="flex items-center gap-2 mb-4">
                <div className="w-8 h-8 bg-primary-600 rounded-lg flex items-center justify-center">
                  <span className="text-white font-bold text-lg">E</span>
                </div>
                <span className="font-bold text-xl text-gray-900 dark:text-white">Enclii</span>
              </div>
              <p className="text-gray-600 dark:text-gray-400 text-sm">
                Deploy without the bill shock.
              </p>
            </div>
            <div>
              <h4 className="font-semibold text-gray-900 dark:text-white mb-4">Product</h4>
              <ul className="space-y-2 text-sm">
                <FooterLink href="https://app.enclii.dev">Dashboard</FooterLink>
                <FooterLink href="https://docs.enclii.dev">Documentation</FooterLink>
                <FooterLink href="https://docs.enclii.dev/changelog">Changelog</FooterLink>
              </ul>
            </div>
            <div>
              <h4 className="font-semibold text-gray-900 dark:text-white mb-4">Resources</h4>
              <ul className="space-y-2 text-sm">
                <FooterLink href="https://github.com/madfam-io/enclii">GitHub</FooterLink>
                <FooterLink href="https://docs.enclii.dev/guides">Guides</FooterLink>
                <FooterLink href="https://docs.enclii.dev/api">API Reference</FooterLink>
              </ul>
            </div>
            <div>
              <h4 className="font-semibold text-gray-900 dark:text-white mb-4">Company</h4>
              <ul className="space-y-2 text-sm">
                <FooterLink href="https://madfam.io">About</FooterLink>
                <FooterLink href="https://status.enclii.dev">Status</FooterLink>
              </ul>
            </div>
          </div>
          <div className="border-t border-gray-200 dark:border-gray-800 mt-12 pt-8 text-center text-gray-600 dark:text-gray-400 text-sm">
            <p>&copy; {new Date().getFullYear()} Madfam. All rights reserved.</p>
          </div>
        </div>
      </footer>
    </main>
  )
}

function FeatureCard({ icon, title, description }: { icon: React.ReactNode; title: string; description: string }) {
  return (
    <div className="feature-card bg-white dark:bg-gray-800 p-6 rounded-xl shadow-lg border border-gray-200 dark:border-gray-700">
      <div className="w-14 h-14 bg-primary-100 dark:bg-primary-900/30 text-primary-600 dark:text-primary-400 rounded-xl flex items-center justify-center mb-4">
        {icon}
      </div>
      <h3 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">{title}</h3>
      <p className="text-gray-600 dark:text-gray-400">{description}</p>
    </div>
  )
}

function BenefitRow({ text }: { text: string }) {
  return (
    <div className="flex items-center gap-4 bg-white dark:bg-gray-800 p-4 rounded-xl border border-gray-200 dark:border-gray-700">
      <div className="w-8 h-8 bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400 rounded-full flex items-center justify-center flex-shrink-0">
        <Check className="w-5 h-5" />
      </div>
      <span className="text-gray-900 dark:text-white">{text}</span>
    </div>
  )
}

function FooterLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <li>
      <a href={href} className="text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors">
        {children}
      </a>
    </li>
  )
}
