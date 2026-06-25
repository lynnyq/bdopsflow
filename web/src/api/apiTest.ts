import api from '@/utils/api'
import type { AxiosRequestConfig } from 'axios'

// 接口测试用例
export interface ApiTest {
  id: number
  name: string
  type: 'http' | 'grpc'
  config: string
  created_by: number
  created_at: string
  updated_at: string
}

// 接口测试执行结果
export interface ApiTestResult {
  id: number
  test_id: number
  type: 'http' | 'grpc'
  status_code: number
  latency_ms: number
  headers: string
  body: string
  error: string
  assertions_result: string
  executed_by: number
  executed_by_name?: string
  created_at: string
  test_name?: string
}

// HTTP 请求配置
export interface HTTPRequestConfig {
  method: string
  url: string
  headers?: { key: string; value: string }[]
  params?: { key: string; value: string }[]
  body?: HTTPBodyConfig
  auth?: HTTPAuthConfig
  pre_script?: string
  post_script?: string
  timeout?: number
}

export interface HTTPBodyConfig {
  type: 'none' | 'json' | 'form-urlencoded' | 'form-multipart' | 'raw' | 'binary'
  content: string
}

export interface HTTPAuthConfig {
  type: 'none' | 'bearer' | 'basic' | 'apikey'
  token?: string
  user?: string
  pass?: string
  key?: string
  value?: string
  in?: 'header' | 'query'
}

// gRPC 请求配置
export interface GRPCRequestConfig {
  address: string
  service: string
  method: string
  request_body: string
  metadata?: { key: string; value: string }[]
  tls_mode: 'insecure' | 'tls' | 'mtls'
  certificate_id?: number | null
  proto_file_id?: number | null
  use_reflection: boolean
  timeout?: number
}

// 断言配置
export interface AssertionConfig {
  type: 'status_code' | 'json_path' | 'header'
  target: string
  operator: 'equals' | 'not_equals' | 'contains' | 'gt' | 'lt' | 'exists'
  expected: string
}

export interface AssertionResult {
  assertion: AssertionConfig
  passed: boolean
  actual: string
  message?: string
}

// Proto 文件
export interface ProtoFile {
  id: number
  name: string
  content: string
  file_hash: string
  parsed_result?: string
  dependencies: string
  created_by: number
  created_by_name: string
  created_at: string
  updated_at: string
}

export interface ProtoService {
  name: string
  methods: ProtoMethod[]
}

export interface ProtoMethod {
  name: string
  input_type: string
  output_type: string
  client_stream: boolean
  server_stream: boolean
}

// Proto 消息字段定义
export interface ProtoMessageField {
  name: string
  number: number
  type: string              // "string", "int32", "bool", "message:FullTypeName", "enum:FullEnumName", etc.
  label: string             // "optional", "repeated", "map"
  map_key?: string
  map_value?: string
  fields?: ProtoMessageField[] // nested message fields
}

// Proto 消息定义
export interface ProtoMessageDef {
  name: string
  full_name: string
  fields: ProtoMessageField[]
}

export interface ProtoParseResult {
  package?: string
  services?: ProtoService[]
  messages?: string[]
}

// 证书
export interface Certificate {
  id: number
  name: string
  ca_cert?: string
  client_cert?: string
  client_key?: string
  created_by: number
  created_at: string
  updated_at: string
}

export interface CertificateSummary {
  id: number
  name: string
  has_ca_cert: boolean
  has_client_cert: boolean
  has_client_key: boolean
  created_by: number
  created_by_name: string
  created_at: string
  updated_at: string
}

// 接口测试 API
export const apiTestAPI = {
  list: (params?: { type?: string; page?: number; page_size?: number }) =>
    api.get<{ items: ApiTest[]; total: number; page: number; page_size: number }>('/interfaces', { params }),
  get: (id: number) =>
    api.get<ApiTest>(`/interfaces/${id}`),
  create: (data: { name: string; type: string; config: string }) =>
    api.post<{ id: number }>('/interfaces', data),
  update: (id: number, data: { name?: string; type?: string; config?: string }) =>
    api.put(`/interfaces/${id}`, data),
  delete: (id: number) =>
    api.delete(`/interfaces/${id}`),
  execute: (data: { type: string; config: string; save_result?: boolean; assertions?: AssertionConfig[] }, axiosConfig?: AxiosRequestConfig) =>
    api.post<ApiTestResult>('/interfaces/execute', data, { timeout: 310000, ...axiosConfig }),
  executeSaved: (id: number, data?: { assertions?: AssertionConfig[] }, axiosConfig?: AxiosRequestConfig) =>
    api.post<ApiTestResult>(`/interfaces/${id}/execute`, data, { timeout: 310000, ...axiosConfig }),
  getResults: (id: number, params?: { page?: number; page_size?: number }) =>
    api.get<{ items: ApiTestResult[]; total: number; page: number; page_size: number }>(`/interfaces/${id}/results`, { params }),
  listResults: (params?: { type?: string; page?: number; page_size?: number }) =>
    api.get<{ items: ApiTestResult[]; total: number; page: number; page_size: number }>('/interfaces/results', { params }),
  deleteResult: (id: number) =>
    api.delete(`/interfaces/results/${id}`),
  generateCurl: (data: HTTPRequestConfig) =>
    api.post<{ curl: string }>('/interfaces/generate-curl', data),
}

// Proto 文件 API
export const protoFileAPI = {
  list: (params?: { page?: number; page_size?: number }) =>
    api.get<{ items: ProtoFile[]; total: number; page: number; page_size: number }>('/proto-files', { params }),
  get: (id: number) =>
    api.get<ProtoFile>(`/proto-files/${id}`),
  create: (data: { name: string; content: string; dependencies?: number[] }) =>
    api.post('/proto-files', data),
  update: (id: number, data: { name?: string; content?: string; dependencies?: number[] }) =>
    api.put(`/proto-files/${id}`, data),
  delete: (id: number) =>
    api.delete(`/proto-files/${id}`),
  parse: (data: { content: string; dependencies?: string[] }) =>
    api.post<ProtoParseResult>('/proto-files/parse', data),
  reflect: (data: { address: string; tls_mode?: string; certificate_id?: number }) =>
    api.post<{ services: ProtoService[] }>('/proto-files/reflect', data, { timeout: 30000 }),
  template: (data: { proto_file_id: number; service: string; method: string }) =>
    api.post<{ template: string }>('/proto-files/template', data),
  fields: (data: { proto_file_id: number }) =>
    api.post<{ messages: ProtoMessageDef[] }>('/proto-files/fields', data),
}

// 证书 API
export const certificateAPI = {
  list: (params?: { page?: number; page_size?: number }) =>
    api.get<{ items: CertificateSummary[]; total: number; page: number; page_size: number }>('/certificates', { params }),
  get: (id: number) =>
    api.get<Certificate>(`/certificates/${id}`),
  create: (data: { name: string; ca_cert?: string; client_cert?: string; client_key?: string }) =>
    api.post('/certificates', data),
  update: (id: number, data: { name?: string; ca_cert?: string; client_cert?: string; client_key?: string }) =>
    api.put(`/certificates/${id}`, data),
  delete: (id: number) =>
    api.delete(`/certificates/${id}`),
}
