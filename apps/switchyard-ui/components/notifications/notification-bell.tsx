'use client';

import { useState, useEffect } from 'react';
import { Bell, Check, X, AlertCircle, CheckCircle2, Clock, Rocket } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Button } from '@/components/ui/button';
import Link from 'next/link';

export interface Notification {
  id: string;
  type: 'deployment' | 'build' | 'error' | 'info';
  title: string;
  message: string;
  timestamp: Date;
  read: boolean;
  link?: string;
}

// Mock notifications for now - will be replaced with real API
const mockNotifications: Notification[] = [
  {
    id: '1',
    type: 'deployment',
    title: 'Deployment Completed',
    message: 'switchyard-api deployed to production',
    timestamp: new Date(Date.now() - 5 * 60 * 1000),
    read: false,
    link: '/services/switchyard-api',
  },
  {
    id: '2',
    type: 'build',
    title: 'Build Started',
    message: 'Building janua-docs from commit abc123',
    timestamp: new Date(Date.now() - 15 * 60 * 1000),
    read: false,
    link: '/deployments',
  },
  {
    id: '3',
    type: 'error',
    title: 'Build Failed',
    message: 'demo-app build failed: npm install error',
    timestamp: new Date(Date.now() - 60 * 60 * 1000),
    read: true,
    link: '/deployments',
  },
  {
    id: '4',
    type: 'info',
    title: 'Welcome to Enclii',
    message: 'Start by importing a project from GitHub',
    timestamp: new Date(Date.now() - 24 * 60 * 60 * 1000),
    read: true,
  },
];

function getNotificationIcon(type: Notification['type']) {
  switch (type) {
    case 'deployment':
      return <Rocket className="h-4 w-4 text-green-500" />;
    case 'build':
      return <Clock className="h-4 w-4 text-blue-500" />;
    case 'error':
      return <AlertCircle className="h-4 w-4 text-red-500" />;
    case 'info':
      return <CheckCircle2 className="h-4 w-4 text-blue-500" />;
  }
}

function formatTimestamp(date: Date): string {
  const now = new Date();
  const diff = now.getTime() - date.getTime();

  const minutes = Math.floor(diff / (1000 * 60));
  const hours = Math.floor(diff / (1000 * 60 * 60));
  const days = Math.floor(diff / (1000 * 60 * 60 * 24));

  if (minutes < 1) return 'Just now';
  if (minutes < 60) return `${minutes}m ago`;
  if (hours < 24) return `${hours}h ago`;
  if (days < 7) return `${days}d ago`;
  return date.toLocaleDateString();
}

export function NotificationBell() {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [isOpen, setIsOpen] = useState(false);

  useEffect(() => {
    // In production, this would fetch from API
    setNotifications(mockNotifications);

    // TODO: Set up WebSocket connection for real-time updates
    // const ws = new WebSocket(`${process.env.NEXT_PUBLIC_WS_URL}/notifications`);
    // ws.onmessage = (event) => {
    //   const notification = JSON.parse(event.data);
    //   setNotifications((prev) => [notification, ...prev]);
    // };
    // return () => ws.close();
  }, []);

  const unreadCount = notifications.filter((n) => !n.read).length;

  const markAsRead = (id: string) => {
    setNotifications((prev) =>
      prev.map((n) => (n.id === id ? { ...n, read: true } : n))
    );
  };

  const markAllAsRead = () => {
    setNotifications((prev) => prev.map((n) => ({ ...n, read: true })));
  };

  const dismissNotification = (id: string, e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setNotifications((prev) => prev.filter((n) => n.id !== id));
  };

  return (
    <DropdownMenu open={isOpen} onOpenChange={setIsOpen}>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="h-9 w-9 relative">
          <Bell className="h-4 w-4" />
          {unreadCount > 0 && (
            <span className="absolute -top-1 -right-1 h-5 w-5 flex items-center justify-center rounded-full bg-red-500 text-white text-xs font-medium">
              {unreadCount > 9 ? '9+' : unreadCount}
            </span>
          )}
          <span className="sr-only">
            {unreadCount > 0 ? `${unreadCount} unread notifications` : 'Notifications'}
          </span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-80">
        <div className="flex items-center justify-between px-4 py-2">
          <DropdownMenuLabel className="p-0">Notifications</DropdownMenuLabel>
          {unreadCount > 0 && (
            <button
              onClick={markAllAsRead}
              className="text-xs text-muted-foreground hover:text-foreground flex items-center gap-1"
            >
              <Check className="h-3 w-3" />
              Mark all read
            </button>
          )}
        </div>
        <DropdownMenuSeparator />
        <div className="max-h-96 overflow-y-auto">
          {notifications.length === 0 ? (
            <div className="py-8 text-center text-muted-foreground">
              <Bell className="h-8 w-8 mx-auto mb-2 opacity-50" />
              <p className="text-sm">No notifications</p>
            </div>
          ) : (
            notifications.map((notification) => {
              const content = (
                <div
                  className={`flex items-start gap-3 px-4 py-3 hover:bg-accent cursor-pointer transition-colors ${
                    !notification.read ? 'bg-accent/50' : ''
                  }`}
                  onClick={() => markAsRead(notification.id)}
                >
                  <div className="flex-shrink-0 mt-0.5">
                    {getNotificationIcon(notification.type)}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-start justify-between gap-2">
                      <p className={`text-sm ${!notification.read ? 'font-medium' : ''}`}>
                        {notification.title}
                      </p>
                      <button
                        onClick={(e) => dismissNotification(notification.id, e)}
                        className="flex-shrink-0 text-muted-foreground hover:text-foreground opacity-0 group-hover:opacity-100 transition-opacity"
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </div>
                    <p className="text-xs text-muted-foreground truncate">
                      {notification.message}
                    </p>
                    <p className="text-xs text-muted-foreground mt-1">
                      {formatTimestamp(notification.timestamp)}
                    </p>
                  </div>
                  {!notification.read && (
                    <div className="flex-shrink-0">
                      <span className="h-2 w-2 rounded-full bg-blue-500 block" />
                    </div>
                  )}
                </div>
              );

              if (notification.link) {
                return (
                  <Link
                    key={notification.id}
                    href={notification.link}
                    className="block group"
                    onClick={() => {
                      markAsRead(notification.id);
                      setIsOpen(false);
                    }}
                  >
                    {content}
                  </Link>
                );
              }

              return (
                <div key={notification.id} className="group">
                  {content}
                </div>
              );
            })
          )}
        </div>
        {notifications.length > 0 && (
          <>
            <DropdownMenuSeparator />
            <Link
              href="/activity"
              className="block px-4 py-2 text-sm text-center text-muted-foreground hover:text-foreground hover:bg-accent"
              onClick={() => setIsOpen(false)}
            >
              View all activity
            </Link>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
