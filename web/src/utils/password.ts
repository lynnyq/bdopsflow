/**
 * @file password.ts - 密码加密与校验工具
 * 使用 RSA 公钥（jsencrypt 库）对用户密码进行前端加密后再提交到后端，
 * 同时提供密码复杂度校验函数。公钥由 auth store 在登录前统一加载并缓存。
 */
import JSEncrypt from "jsencrypt";

// 模块级缓存：常规登录 RSA 公钥
let cachedPublicKey: string | null = null;
// 模块级缓存：SSO 登录专用 RSA 公钥
let cachedSSOPublicKey: string | null = null;

/** 密码复杂度规则常量 —— 供注册/修改密码表单统一使用 */
export const PASSWORD_RULES = {
  minLength: 6,        // 最小长度
  maxLength: 30,       // 最大长度
  rules: [
    "长度为 6-30 位字符",
    "必须包含字母和数字",
  ],
}

/**
 * 设置常规登录使用的 RSA 公钥，写入模块级缓存。
 * @param key - PEM 格式的 RSA 公钥字符串
 */
export function setPublicKey(key: string) {
  cachedPublicKey = key;
}

/**
 * 获取当前缓存的常规登录 RSA 公钥。
 * @returns PEM 格式公钥字符串，未加载时返回 null
 */
export function getPublicKey(): string | null {
  return cachedPublicKey;
}

/**
 * 设置 SSO 登录专用的 RSA 公钥。
 * @param key - PEM 格式的 SSO 专用 RSA 公钥
 */
export function setSSOPublicKey(key: string) {
  cachedSSOPublicKey = key;
}

/**
 * 获取当前缓存的 SSO 专用 RSA 公钥。
 * @returns PEM 格式公钥字符串，未加载时返回 null
 */
export function getSSOPublicKey(): string | null {
  return cachedSSOPublicKey;
}

/**
 * 使用常规登录 RSA 公钥对明文密码进行加密，返回 Base64 密文。
 * 加密前需要先调用 setPublicKey 或通过 auth store 的 fetchPublicKey 加载。
 * @param txt - 明文密码
 * @returns RSA 加密后的 Base64 密文
 * @throws 当公钥未加载时抛出 Error("公钥未加载，无法加密密码")
 * @throws 当 jsencrypt 加密失败时抛出 Error("密码加密失败")
 */
export function encryptPassword(txt: string): string {
  if (!cachedPublicKey) {
    throw new Error("公钥未加载，无法加密密码");
  }
  const encryptor = new JSEncrypt();
  encryptor.setPublicKey(cachedPublicKey);
  const encrypted = encryptor.getKey().encrypt(txt);
  if (!encrypted) {
    throw new Error("密码加密失败");
  }
  return encrypted;
}

/**
 * 使用 SSO 专用 RSA 公钥对明文密码进行加密，返回 Base64 密文。
 * 与 encryptPassword 使用不同的密钥对，面向 SSO 后端服务。
 * @param txt - 明文密码
 * @returns RSA 加密后的 Base64 密文
 * @throws 当 SSO 公钥未加载时抛出 Error("SSO公钥未加载，无法加密密码")
 * @throws 当 jsencrypt 加密失败时抛出 Error("SSO密码加密失败")
 */
export function encryptPasswordSSO(txt: string): string {
  if (!cachedSSOPublicKey) {
    throw new Error("SSO公钥未加载，无法加密密码");
  }
  const encryptor = new JSEncrypt();
  encryptor.setPublicKey(cachedSSOPublicKey);
  const encrypted = encryptor.getKey().encrypt(txt);
  if (!encrypted) {
    throw new Error("SSO密码加密失败");
  }
  return encrypted;
}

/**
 * 校验密码是否符合复杂度规则：长度 6~30 位，必须同时包含字母和数字。
 * @param password - 待校验的明文密码
 * @returns { valid, message } 校验结果；valid=true 时 message 为空
 */
export const validatePassword = (password: string): { valid: boolean; message: string } => {
  if (!password || password.length === 0) {
    return { valid: false, message: "请输入密码" }
  }
  if (password.length < PASSWORD_RULES.minLength) {
    return { valid: false, message: `密码长度至少为${PASSWORD_RULES.minLength}位` }
  }
  if (password.length > PASSWORD_RULES.maxLength) {
    return { valid: false, message: `密码长度不能超过${PASSWORD_RULES.maxLength}位` }
  }
  const hasLetter = /[a-zA-Z]/.test(password)
  const hasDigit = /[0-9]/.test(password)
  if (!hasLetter || !hasDigit) {
    return { valid: false, message: "密码必须包含字母和数字" }
  }
  return { valid: true, message: "" }
}
