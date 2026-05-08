import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, ApiError, type MeResponse, type MeUser } from './api'

export const ME_QUERY_KEY = ['me'] as const
export const SETUP_STATUS_QUERY_KEY = ['setup-status'] as const

export function useMe() {
  return useQuery<MeResponse, ApiError, MeUser>({
    queryKey: ME_QUERY_KEY,
    queryFn: api.me,
    select: (r) => r.user,
    retry: false,
    staleTime: 30_000,
  })
}

export function useSetupStatus() {
  return useQuery({
    queryKey: SETUP_STATUS_QUERY_KEY,
    queryFn: api.setupStatus,
    retry: false,
    staleTime: 30_000,
  })
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.login,
    onSuccess: (data) => qc.setQueryData(ME_QUERY_KEY, data),
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.logout,
    onSettled: () => {
      qc.clear()
    },
  })
}

export function useSetupComplete() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.setupComplete,
    onSuccess: (data) => {
      qc.setQueryData(ME_QUERY_KEY, data)
      qc.setQueryData(SETUP_STATUS_QUERY_KEY, { required: false })
    },
  })
}
