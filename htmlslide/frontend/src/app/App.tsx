import React from 'react';
import { RouterProvider } from 'react-router';
import { LocaleProvider } from '../context/LocaleContext';
import { SettingsProvider } from '../hooks/useSettings';
import { router } from './routes';

export default function App() {
  return (
    <LocaleProvider>
      <SettingsProvider>
        <RouterProvider router={router} />
      </SettingsProvider>
    </LocaleProvider>
  );
}
