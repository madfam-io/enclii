import type { Metadata } from 'next'
import './globals.css'

export const metadata: Metadata = {
  title: 'Enclii Dashboard',
  description: 'Control & orchestration for your cloud',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body className="bg-gray-50">
        <nav className="bg-white shadow-sm border-b">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex justify-between h-16">
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  <span className="text-2xl font-bold text-enclii-blue">🚂 Enclii</span>
                </div>
                <div className="ml-10 flex items-baseline space-x-4">
                  <a href="/" className="text-gray-900 hover:text-enclii-blue px-3 py-2 text-sm font-medium">
                    Dashboard
                  </a>
                  <a href="/projects" className="text-gray-500 hover:text-enclii-blue px-3 py-2 text-sm font-medium">
                    Projects
                  </a>
                  <a href="/services" className="text-gray-500 hover:text-enclii-blue px-3 py-2 text-sm font-medium">
                    Services
                  </a>
                </div>
              </div>
            </div>
          </div>
        </nav>
        <main>{children}</main>
      </body>
    </html>
  )
}