import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { gql } from 'graphql-request';
import { graphqlClient } from '@/lib/graphql';

const DEFAULT_CENTER: [number, number] = [2.35, 48.86];
const DEFAULT_ZOOM = 6;

interface MapPosition {
    center: [number, number];
    zoom: number;
    hydrated: boolean; // true once we've attempted to load from backend
}

const MY_MAP_STATE_QUERY = gql`
    query MyMapState {
        myMapState {
            lng
            lat
            zoom
        }
    }
`;

const SAVE_MAP_STATE_MUTATION = gql`
    mutation SaveMapState($lng: Float!, $lat: Float!, $zoom: Float!) {
        saveMapState(lng: $lng, lat: $lat, zoom: $zoom)
    }
`;

interface BackendMapState {
    lng: number;
    lat: number;
    zoom: number;
}

export const fetchMapState = createAsyncThunk('map/fetch', async () => {
    const data = await graphqlClient.request<{ myMapState: BackendMapState | null }>(
        MY_MAP_STATE_QUERY
    );
    return data.myMapState;
});

export const saveMapStateThunk = createAsyncThunk(
    'map/save',
    async ({ lng, lat, zoom }: { lng: number; lat: number; zoom: number }) => {
        await graphqlClient.request(SAVE_MAP_STATE_MUTATION, { lng, lat, zoom });
    }
);

const mapSlice = createSlice({
    name: 'map',
    initialState: {
        center: DEFAULT_CENTER,
        zoom: DEFAULT_ZOOM,
        hydrated: false,
    } as MapPosition,
    reducers: {
        setMapState(state, action: PayloadAction<{ center: [number, number]; zoom: number }>) {
            state.center = action.payload.center;
            state.zoom = action.payload.zoom;
        },
    },
    extraReducers: (builder) => {
        builder
            .addCase(fetchMapState.fulfilled, (state, action) => {
                if (action.payload) {
                    state.center = [action.payload.lng, action.payload.lat];
                    state.zoom = action.payload.zoom;
                }
                state.hydrated = true;
            })
            .addCase(fetchMapState.rejected, (state) => {
                state.hydrated = true;
            });
    },
});

export const { setMapState } = mapSlice.actions;
export default mapSlice.reducer;
