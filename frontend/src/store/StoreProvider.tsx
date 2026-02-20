'use client';

import { useEffect } from 'react';
import { Provider } from 'react-redux';
import { store } from '@/store';
import { fetchMe } from '@/store/authSlice';

function AuthInit({ children }: { children: React.ReactNode }) {
    useEffect(() => {
        store.dispatch(fetchMe());
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
            <AuthInit>{children}</AuthInit>
        </Provider>
    );
}
