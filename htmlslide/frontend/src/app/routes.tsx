import React from 'react';
import { createBrowserRouter, Outlet } from 'react-router';
import { HomeView } from '../views/HomeView';
import { EditorView } from '../views/EditorView';
import { APP_CONFIG } from './config';
import { AppErrorBoundary } from '../components/AppErrorBoundary';

function Root() {
  return <Outlet />;
}

export const router = createBrowserRouter([
  {
    path: APP_CONFIG.routes.home,
    Component: Root,
    ErrorBoundary: AppErrorBoundary,
    children: [
      { index: true, Component: HomeView, ErrorBoundary: AppErrorBoundary },
      {
        path: APP_CONFIG.routes.editor.slice(1),
        Component: EditorView,
        ErrorBoundary: AppErrorBoundary,
      },
      {
        path: `${APP_CONFIG.routes.editor.slice(1)}/:projectId`,
        Component: EditorView,
        ErrorBoundary: AppErrorBoundary,
      },
    ],
  },
]);
