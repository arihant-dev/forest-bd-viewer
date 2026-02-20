import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { graphqlClient } from '@/lib/graphql';
import { gql } from 'graphql-request';

export interface User {
  id: string;
  email: string;
  name: string;
}

interface AuthState {
  user: User | null;
  loading: boolean;
}

const initialState: AuthState = {
  user: null,
  loading: true,
};

const ME_QUERY = gql`
  query Me {
    me {
      id
      email
      name
    }
  }
`;

export const fetchMe = createAsyncThunk('auth/fetchMe', async () => {
  const data = await graphqlClient.request<{ me: User | null }>(ME_QUERY);
  return data.me;
});

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    setUser(state, action: PayloadAction<User>) {
      state.user = action.payload;
      state.loading = false;
    },
    clearUser(state) {
      state.user = null;
      state.loading = false;
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchMe.pending, (state) => {
        state.loading = true;
      })
      .addCase(fetchMe.fulfilled, (state, action) => {
        state.user = action.payload;
        state.loading = false;
      })
      .addCase(fetchMe.rejected, (state) => {
        state.user = null;
        state.loading = false;
      });
  },
});

export const { setUser, clearUser } = authSlice.actions;
export default authSlice.reducer;
