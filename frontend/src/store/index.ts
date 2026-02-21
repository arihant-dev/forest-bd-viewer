import { configureStore } from '@reduxjs/toolkit';
import { TypedUseSelectorHook, useDispatch, useSelector } from 'react-redux';
import authReducer from '@/store/authSlice';
import mapReducer from '@/store/mapSlice';
import analysisReducer from '@/store/analysisSlice';

export const store = configureStore({
    reducer: {
        auth: authReducer,
        map: mapReducer,
        analysis: analysisReducer,
    },
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;

export const useAppDispatch: () => AppDispatch = useDispatch;
export const useAppSelector: TypedUseSelectorHook<RootState> = useSelector;
