import JSEncrypt from "jsencrypt";

let cachedPublicKey: string | null = null;
let cachedSSOPublicKey: string | null = null;

export const PASSWORD_RULES = {
  minLength: 6,
  maxLength: 30,
  rules: [
    "长度为 6-30 位字符",
    "必须包含字母和数字",
  ],
}

export function setPublicKey(key: string) {
  cachedPublicKey = key;
}

export function getPublicKey(): string | null {
  return cachedPublicKey;
}

export function setSSOPublicKey(key: string) {
  cachedSSOPublicKey = key;
}

export function getSSOPublicKey(): string | null {
  return cachedSSOPublicKey;
}

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
