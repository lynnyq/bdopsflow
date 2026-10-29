/**
 * @file apiTest.ts - 接口测试、Proto 文件、证书管理 API 封装
 *
 * 包含三个 API 对象：
 *  - apiTestAPI：HTTP / gRPC 接口测试用例的 CRUD、执行、执行结果查询
 *  - protoFileAPI：Proto 文件的 CRUD、解析、反射、模板生成、字段提取
 *  - certificateAPI：mTLS 证书（CA / 客户端证书 / 私钥）的 CRUD
 *
 * 定义了该模块涉及的所有数据模型接口。
 */
import api from '@/utils/api'
import type { AxiosRequestConfig } from 'axios'

/** 接口测试用例（持久化到数据库） */
export interface ApiTest {
  id: number
  name: string
  type: 'http' | 'grpc'
  config: string
  created_by: number
  created_at: string
  updated_at: string
}

/** 接口测试执行结果 */
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

/** HTTP 请求配置（接口测试模块） */
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

/** HTTP 请求体配置 */
export interface HTTPBodyConfig {
  type: 'none' | 'json' | 'form-urlencoded' | 'form-multipart' | 'raw' | 'binary'
  content: string
}

/** HTTP 认证配置 */
export interface HTTPAuthConfig {
  type: 'none' | 'bearer' | 'basic' | 'apikey'
  token?: string
  user?: string
  pass?: string
  key?: string
  value?: string
  in?: 'header' | 'query'
}

/** gRPC 请求配置（接口测试模块） */
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

/** 断言配置（用于接口测试执行后校验响应） */
export interface AssertionConfig {
  type: 'status_code' | 'json_path' | 'header'
  target: string
  operator: 'equals' | 'not_equals' | 'contains' | 'gt' | 'lt' | 'exists'
  expected: string
}

/** 断言执行结果 */
export interface AssertionResult {
  assertion: AssertionConfig
  passed: boolean
  actual: string
  message?: string
}

/** Proto 文件（用户上传，用于 gRPC 接口测试的序列化） */
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

/** Proto 中的服务定义 */
export interface ProtoService {
  name: string
  methods: ProtoMethod[]
}

/** Proto 中的方法定义 */
export interface ProtoMethod {
  name: string
  input_type: string
  output_type: string
  client_stream: boolean
  server_stream: boolean
}

/** Proto 消息字段定义（可能为嵌套消息） */
export interface ProtoMessageField {
  name: string
  number: number
  type: string              // "string", "int32", "bool", "message:FullTypeName", "enum:FullEnumName", etc.
  label: string             // "optional", "repeated", "map"
  map_key?: string
  map_value?: string
  fields?: ProtoMessageField[] // nested message fields
}

/** Proto 消息定义 */
export interface ProtoMessageDef {
  name: string
  full_name: string
  fields: ProtoMessageField[]
}

/** Proto 文件解析结果 */
export interface ProtoParseResult {
  package?: string
  services?: ProtoService[]
  messages?: string[]
}

/** mTLS 证书完整内容（包含私钥，仅获取详情时返回） */
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

/** 证书摘要（列表页使用，不含私钥内容） */
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

/** 接口测试 API 封装对象（覆盖 HTTP 和 gRPC 两种类型的测试） */
export const apiTestAPI = {
  /** 分页查询测试用例列表 */
  list: (params?: { type?: string; page?: number; page_size?: number }) =>
    api.get<{ items: ApiTest[]; total: number; page: number; page_size: number }>('/interfaces', { params }),
  /** 获取单个测试用例详情 */
  get: (id: number) =>
    api.get<ApiTest>(`/interfaces/${id}`),
  /** 创建新的测试用例 */
  create: (data: { name: string; type: string; config: string }) =>
    api.post<{ id: number }>('/interfaces', data),
  /** 更新测试用例（可修改名称、类型或配置） */
  update: (id: number, data: { name?: string; type?: string; config?: string }) =>
    api.put(`/interfaces/${id}`, data),
  /** 删除测试用例 */
  delete: (id: number) =>
    api.delete(`/interfaces/${id}`),
  /**
   * 即时执行一次接口测试（不保存为用例），默认 5 分钟超时。
   * @param data - 测试配置（type + config），可选附带断言
   * @param axiosConfig - 透传给 axios 的配置（如自定义 timeout）
   */
  execute: (data: { type: string; config: string; save_result?: boolean; assertions?: AssertionConfig[] }, axiosConfig?: AxiosRequestConfig) =>
    api.post<ApiTestResult>('/interfaces/execute', data, { timeout: 310000, ...axiosConfig }),
  /**
   * 执行已保存的测试用例。
   * @param id - 测试用例 ID
   * @param data - 可选的断言覆盖
   */
  executeSaved: (id: number, data?: { assertions?: AssertionConfig[] }, axiosConfig?: AxiosRequestConfig) =>
    api.post<ApiTestResult>(`/interfaces/${id}/execute`, data, { timeout: 310000, ...axiosConfig }),
  /** 查询指定用例的执行历史结果列表 */
  getResults: (id: number, params?: { page?: number; page_size?: number }) =>
    api.get<{ items: ApiTestResult[]; total: number; page: number; page_size: number }>(`/interfaces/${id}/results`, { params }),
  /** 全局查询执行历史结果（支持类型、名称、状态过滤） */
  listResults: (params?: { type?: string; search?: string; status?: string; page?: number; page_size?: number }) =>
    api.get<{ items: ApiTestResult[]; total: number; page: number; page_size: number }>('/interfaces/results', { params }),
  /** 删除某条执行结果 */
  deleteResult: (id: number) =>
    api.delete(`/interfaces/results/${id}`),
  /** 根据 HTTP 请求配置生成等价的 curl 命令字符串 */
  generateCurl: (data: HTTPRequestConfig) =>
    api.post<{ curl: string }>('/interfaces/generate-curl', data),
}

/** Proto 文件管理 API 封装对象 */
export const protoFileAPI = {
  /** 分页查询 Proto 文件列表 */
  list: (params?: { page?: number; page_size?: number; search?: string }) =>
    api.get<{ items: ProtoFile[]; total: number; page: number; page_size: number }>('/proto-files', { params }),
  /** 获取单个 Proto 文件详情（含 content） */
  get: (id: number) =>
    api.get<ProtoFile>(`/proto-files/${id}`),
  /** 上传并创建新的 Proto 文件 */
  create: (data: { name: string; content: string; dependencies?: number[] }) =>
    api.post('/proto-files', data),
  /** 更新 Proto 文件的名称、内容或依赖 */
  update: (id: number, data: { name?: string; content?: string; dependencies?: number[] }) =>
    api.put(`/proto-files/${id}`, data),
  /** 删除 Proto 文件 */
  delete: (id: number) =>
    api.delete(`/proto-files/${id}`),
  /**
   * 解析 Proto 内容，返回 package、services、messages 列表。
   * 用于前端预览或编辑时快速验证语法。
   */
  parse: (data: { content: string; dependencies?: string[] }) =>
    api.post<ProtoParseResult>('/proto-files/parse', data),
  /**
   * 使用 gRPC 反射从在线服务获取服务定义（无需 Proto 文件）。
   * 默认 30s 超时以适配网络抖动。
   */
  reflect: (data: { address: string; tls_mode?: string; certificate_id?: number }) =>
    api.post<{ services: ProtoService[] }>('/proto-files/reflect', data, { timeout: 30000 }),
  /** 根据 Proto 文件的 service/method 生成请求体 JSON 模板 */
  template: (data: { proto_file_id: number; service: string; method: string }) =>
    api.post<{ template: string }>('/proto-files/template', data),
  /** 获取 Proto 文件中所有 message 及其字段（用于表单自动生成） */
  fields: (data: { proto_file_id: number }) =>
    api.post<{ messages: ProtoMessageDef[] }>('/proto-files/fields', data),
}

/** mTLS 证书管理 API 封装对象 */
export const certificateAPI = {
  /** 分页查询证书列表（返回摘要，不含私钥） */
  list: (params?: { page?: number; page_size?: number; search?: string }) =>
    api.get<{ items: CertificateSummary[]; total: number; page: number; page_size: number }>('/certificates', { params }),
  /** 获取证书详情（含完整 CA/客户端证书/私钥内容） */
  get: (id: number) =>
    api.get<Certificate>(`/certificates/${id}`),
  /** 创建新证书（可只上传部分证书类型） */
  create: (data: { name: string; ca_cert?: string; client_cert?: string; client_key?: string }) =>
    api.post('/certificates', data),
  /** 更新证书内容（可逐项覆盖） */
  update: (id: number, data: { name?: string; ca_cert?: string; client_cert?: string; client_key?: string }) =>
    api.put(`/certificates/${id}`, data),
  /** 删除证书 */
  delete: (id: number) =>
    api.delete(`/certificates/${id}`),
}
