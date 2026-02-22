'use client';

import { useEffect } from 'react';
import { Provider } from 'react-redux';
import { store } from '@/store';
import { fetchMe } from '@/store/authSlice';
import { fetchMapState } from '@/store/mapSlice';
import { I18nProvider } from '@/lib/i18n';
import { ThemeProvider } from '@/lib/theme';

function AppInit({ children }: { children: React.ReactNode }) {
    useEffect(() => {
        store.dispatch(fetchMe());
        store.dispatch(fetchMapState());
    }, []);
    return <>{children}</>;
}

export default function StoreProvider({
    children,
}: {
    children: React.ReactNode;
}) {
    return (
        <Provider store={store}>
            <I18nProvider>
                <ThemeProvider>
                    <AppInit>{children}</AppInit>
                </ThemeProvider>
            </I18nProvider>
        </Provider>
    );
}
