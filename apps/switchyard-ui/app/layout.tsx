'use client';

import type { Metadata } from 'next'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import './globals.css'

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const pathname = usePathname()

  const navigation = [
    { name: 'Dashboard', href: '/' },
    { name: 'Projects', href: '/projects' },
    { name: 'Services', href: '/services' },
    { name: 'Deployments', href: '/deployments' },
  ]

  return (
    <html lang="en">
      <body className="bg-gray-50">
        <nav className="bg-white shadow-sm border-b">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex justify-between h-16">
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  <Link href="/" className="flex items-center">
                    <span className="text-2xl font-bold text-enclii-blue">🚂 Enclii</span>
                    <span className="ml-2 text-sm text-gray-500 font-medium">Switchyard</span>
                  </Link>
                </div>
                <div className="ml-10 flex items-baseline space-x-4">
                  {navigation.map((item) => {
                    const isActive = pathname === item.href || (item.href !== '/' && pathname.startsWith(item.href))
                    return (
                      <Link
                        key={item.name}
                        href={item.href}
                        className={`px-3 py-2 text-sm font-medium transition-colors duration-150 ${
                          isActive
                            ? 'text-enclii-blue border-b-2 border-enclii-blue'
                            : 'text-gray-500 hover:text-enclii-blue hover:border-b-2 hover:border-gray-300'
                        }`}
                      >
                        {item.name}
                      </Link>
                    )
                  })}
                </div>
              </div>
              <div className="flex items-center space-x-4">
                <div className="flex items-center text-sm text-gray-500">
                  <div className="w-2 h-2 bg-green-500 rounded-full mr-2"></div>
                  <span>System Healthy</span>
                </div>
                <button className="bg-gray-100 p-2 rounded-full text-gray-600 hover:text-gray-900 hover:bg-gray-200 transition-colors duration-150">
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                  </svg>
                </button>
              </div>
            </div>
          </div>
        </nav>
        <main className="min-h-screen">{children}</main>
        <footer className="bg-white border-t mt-12">
          <div className="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
            <div className="flex items-center justify-between">
              <div className="text-sm text-gray-500">
                © 2024 Enclii Platform. Built with ❤️ for developers.
              </div>
              <div className="flex items-center space-x-4 text-sm text-gray-500">
                <a href="#" className="hover:text-gray-700">Documentation</a>
                <a href="#" className="hover:text-gray-700">API</a>
                <a href="#" className="hover:text-gray-700">Status</a>
              </div>
            </div>
          </div>
        </footer>
      </body>
    </html>
  )
}