"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { LoaderCircle, MailCheck, Sparkles } from "lucide-react";
import { toast } from "sonner";

import { TurnstileWidget } from "@/components/turnstile-widget";
import {
  fetchRegistrationOptions,
  login,
  registerBusinessUser,
  requestRegistrationCode,
  type LoginResult,
  type RegistrationOptions,
} from "@/lib/api";
import { usePublicSiteSettings, usePublicTurnstileSettings } from "@/lib/site-settings";
import { setStoredAuthAvatarUrl, setStoredAuthRole, setStoredAuthUsername } from "@/store/auth";

export type AuthMode = "login" | "register";

type AuthCardProps = {
  mode: AuthMode;
  onModeChange: (mode: AuthMode) => void;
  from?: string;
  syncUrl?: boolean;
  className?: string;
};

export function AuthCard({
  mode,
  onModeChange,
  from = "",
  syncUrl = false,
  className,
}: AuthCardProps) {
  const navigate = useNavigate();
  const site = usePublicSiteSettings();
  const turnstile = usePublicTurnstileSettings();
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [code, setCode] = useState("");
  const [registerCode, setRegisterCode] = useState("");
  const [affiliateCode, setAffiliateCode] = useState("");
  const [turnstileToken, setTurnstileToken] = useState("");
  const [turnstileResetKey, setTurnstileResetKey] = useState(0);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSendingCode, setIsSendingCode] = useState(false);
  const [codeCooldownLeft, setCodeCooldownLeft] = useState(0);
  const [registrationOptions, setRegistrationOptions] =
    useState<RegistrationOptions | null>(null);

  useEffect(() => {
    let cancelled = false;
    const loadRegistrationOptions = async () => {
      try {
        const result = await fetchRegistrationOptions();
        if (!cancelled) {
          setRegistrationOptions(result);
        }
      } catch {
        if (!cancelled) {
          setRegistrationOptions({
            enabled: false,
            registration: false,
            emailVerificationConfigured: false,
            codeTTLSeconds: 600,
            codeCooldownSeconds: 60,
            registrationCodeRequired: false,
          });
        }
      }
    };
    void loadRegistrationOptions();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const aff = params.get("aff") || "";
    if (aff.trim()) {
      setAffiliateCode(aff.trim());
    }
  }, []);

  useEffect(() => {
    if (codeCooldownLeft <= 0) {
      return;
    }
    const timer = window.setInterval(() => {
      setCodeCooldownLeft((current) => Math.max(0, current - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [codeCooldownLeft]);

  const registrationOpen = Boolean(registrationOptions?.registration);
  const registrationCodeRequired = Boolean(registrationOptions?.registrationCodeRequired);
  const canSendCode = !isSendingCode && codeCooldownLeft <= 0;
  const formDisabled = useMemo(
    () => isSubmitting || isSendingCode,
    [isSubmitting, isSendingCode],
  );
  const loginTurnstileRequired = Boolean(turnstile.enabled && turnstile.siteKey && turnstile.login);
  const registerCodeTurnstileRequired = Boolean(turnstile.enabled && turnstile.siteKey && turnstile.registerCode);
  const registerSubmitTurnstileRequired = Boolean(turnstile.enabled && turnstile.siteKey && turnstile.registerSubmit);
  const visibleTurnstileRequired = mode === "login" ? loginTurnstileRequired : registerSubmitTurnstileRequired;
  const showRegisterCodeTurnstile = mode === "register" && registerCodeTurnstileRequired && !registerSubmitTurnstileRequired && codeCooldownLeft <= 0;
  const resetTurnstile = () => {
    setTurnstileToken("");
    setTurnstileResetKey((current) => current + 1);
  };
  const authTitle = mode === "register" ? "创建你的账号" : `登录 ${site.name || "Poom Studio"}`;
  const authSubtitle =
    mode === "register"
      ? "创建你的 AI 图像工作区，开始沉淀提示词、素材和生成结果。"
      : "登录后继续使用生图工作台、资产库和后台管理。";

  const completeLogin = async (result: LoginResult, fallbackEmail: string) => {
    await setStoredAuthRole(result.role);
    await setStoredAuthUsername(result.username || result.email || fallbackEmail);
    await setStoredAuthAvatarUrl(result.avatarUrl || null);
    const fallback = result.role === "admin" ? "/admin/dashboard" : "/image/history";
    navigate(from && from !== "/login" ? from : fallback, { replace: true });
  };

  const switchMode = (nextMode: AuthMode) => {
    onModeChange(nextMode);
    if (syncUrl) {
      navigate(nextMode === "register" ? "/login?mode=register" : "/login", {
        replace: true,
      });
    }
  };

  const handleOpenRegister = () => {
    if (!registrationOptions) {
      toast.error("正在读取注册状态，请稍后再试");
      return;
    }
    if (!registrationOpen) {
      toast.error("未开放注册");
      return;
    }
    switchMode("register");
  };

  const handleLogin = async () => {
    const normalizedEmail = email.trim();
    if (!normalizedEmail || !password.trim()) {
      toast.error("请输入邮箱和密码");
      return;
    }
    if (loginTurnstileRequired && !turnstileToken) {
      toast.error("请先完成人机验证");
      return;
    }

    setIsSubmitting(true);
    try {
      const result = await login(normalizedEmail, password, turnstileToken);
      await completeLogin(result, normalizedEmail);
    } catch (error) {
      resetTurnstile();
      toast.error(error instanceof Error ? error.message : "登录失败");
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleSendCode = async () => {
    const normalizedEmail = email.trim();
    if (!canSendCode) {
      return;
    }
    if (!registrationOpen) {
      toast.error("未开放注册");
      return;
    }
    if (!normalizedEmail) {
      toast.error("请输入邮箱");
      return;
    }
    if (registerCodeTurnstileRequired && !turnstileToken) {
      toast.error("请先完成人机验证");
      return;
    }
    setIsSendingCode(true);
    try {
      const result = await requestRegistrationCode(normalizedEmail, turnstileToken);
      resetTurnstile();
      setCodeCooldownLeft(
        Math.max(
          1,
          result.cooldownSeconds || registrationOptions?.codeCooldownSeconds || 60,
        ),
      );
      toast.success("验证码已发送，请检查邮箱");
    } catch (error) {
      resetTurnstile();
      toast.error(error instanceof Error ? error.message : "发送验证码失败");
    } finally {
      setIsSendingCode(false);
    }
  };

  const handleRegister = async () => {
    const normalizedEmail = email.trim();
    const normalizedUsername = username.trim();
    if (!registrationOpen) {
      toast.error("未开放注册");
      return;
    }
    if (!normalizedEmail || !password.trim() || !confirmPassword.trim() || !code.trim()) {
      toast.error("请输入邮箱、密码、确认密码和验证码");
      return;
    }
    if (registrationCodeRequired && !registerCode.trim()) {
      toast.error("请输入注册码");
      return;
    }
    if (password.trim().length < 6) {
      toast.error("密码至少 6 位");
      return;
    }
    if (password !== confirmPassword) {
      toast.error("两次输入的密码不一致");
      return;
    }
    if (registerSubmitTurnstileRequired && !turnstileToken) {
      toast.error("请先完成人机验证");
      return;
    }
    setIsSubmitting(true);
    try {
      const result = await registerBusinessUser({
        email: normalizedEmail,
        username: normalizedUsername || undefined,
        password,
        code,
        registerCode,
        affiliateCode,
        turnstileToken,
      });
      toast.success("注册成功");
      await completeLogin(result, normalizedEmail);
    } catch (error) {
      resetTurnstile();
      toast.error(error instanceof Error ? error.message : "注册失败");
    } finally {
      setIsSubmitting(false);
    }
  };

  const submit = () => (mode === "register" ? handleRegister() : handleLogin());

  return (
    <section className={`auth-card${className ? ` ${className}` : ""}`}>
      <div className="auth-card-inner">
        <div className="auth-brand-row">
          <AuthBrandMark logoUrl={site.logoUrl} siteName={site.name} compact />
          <span>{site.name || "Poom Studio"}</span>
        </div>

        <h2>{authTitle}</h2>
        <p className="auth-sub">{authSubtitle}</p>

        <div className="auth-fields auth-fields-anim" key={mode}>
          <AuthField label="邮箱" htmlFor="email">
            <input
              id="email"
              name="email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  void submit();
                }
              }}
              placeholder="you@example.com"
              className="auth-input"
            />
          </AuthField>

          {mode === "register" ? (
            <AuthField label="用户名" htmlFor="username">
              <input
                id="username"
                name="username"
                type="text"
                autoComplete="username"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                placeholder="你的名称（选填）"
                className="auth-input"
              />
            </AuthField>
          ) : null}

          <AuthField label="密码" htmlFor="password">
            <input
              id="password"
              name="password"
              type="password"
              autoComplete={mode === "register" ? "new-password" : "current-password"}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  void submit();
                }
              }}
              placeholder="请输入密码"
              className="auth-input"
            />
          </AuthField>

          {mode === "register" ? (
            <>
              <AuthField label="确认密码" htmlFor="confirm-password">
                <input
                  id="confirm-password"
                  name="confirm-password"
                  type="password"
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      void handleRegister();
                    }
                  }}
                  placeholder="请再次输入密码"
                  className="auth-input"
                />
              </AuthField>

              <AuthField label="验证码" htmlFor="verification-code">
                <div className="auth-code-row">
                  <input
                    id="verification-code"
                    name="verification-code"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    value={code}
                    onChange={(event) => setCode(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        void handleRegister();
                      }
                    }}
                    placeholder="邮箱验证码"
                    className="auth-input"
                  />
                  <button
                    type="button"
                    className="auth-code-send"
                    onClick={() => void handleSendCode()}
                    disabled={!canSendCode || formDisabled}
                  >
                    {isSendingCode ? (
                      <LoaderCircle className="size-4 animate-spin" />
                    ) : (
                      <MailCheck className="size-4" />
                    )}
                    {codeCooldownLeft > 0 ? `${codeCooldownLeft}s` : "发送"}
                  </button>
                </div>
              </AuthField>

              {registrationCodeRequired ? (
                <AuthField label="注册码" htmlFor="register-code">
                  <input
                    id="register-code"
                    name="register-code"
                    value={registerCode}
                    onChange={(event) => setRegisterCode(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        void handleRegister();
                      }
                    }}
                    placeholder="邀请码或优惠码"
                    className="auth-input"
                  />
                </AuthField>
              ) : null}
            </>
          ) : null}

          {showRegisterCodeTurnstile ? (
            <TurnstileWidget
              enabled
              siteKey={turnstile.siteKey}
              action="register-code"
              disabled={formDisabled}
              resetKey={turnstileResetKey}
              onTokenChange={setTurnstileToken}
            />
          ) : (
            <TurnstileWidget
              enabled={visibleTurnstileRequired}
              siteKey={turnstile.siteKey}
              action={mode}
              disabled={formDisabled}
              resetKey={turnstileResetKey}
              onTokenChange={setTurnstileToken}
            />
          )}

          <button
            type="button"
            className="auth-primary"
            onClick={() => void submit()}
            disabled={isSubmitting || (visibleTurnstileRequired && !turnstileToken)}
          >
            {isSubmitting ? <LoaderCircle className="size-4 animate-spin" /> : null}
            {mode === "register" ? "注册并登录" : "登录"}
          </button>

          {mode === "login" ? (
            <div className="auth-forgot">
              <Link to="/forgot-password">忘记密码？</Link>
            </div>
          ) : null}

          <p className="auth-switch">
            {mode === "register" ? (
              <>
                已有账号？
                <button type="button" onClick={() => switchMode("login")}>返回登录</button>
              </>
            ) : (
              <>
                还没有账户？
                <button type="button" onClick={handleOpenRegister}>创建账号</button>
              </>
            )}
          </p>
        </div>
      </div>
    </section>
  );
}

function AuthField({
  label,
  htmlFor,
  children,
}: {
  label: string;
  htmlFor: string;
  children: ReactNode;
}) {
  return (
    <div className="auth-field">
      <label htmlFor={htmlFor}>{label}</label>
      {children}
    </div>
  );
}

export function AuthBrandMark({
  logoUrl,
  siteName,
  compact = false,
}: {
  logoUrl?: string;
  siteName: string;
  compact?: boolean;
}) {
  const [failed, setFailed] = useState(false);
  const size = compact ? 30 : 32;

  useEffect(() => {
    setFailed(false);
  }, [logoUrl]);

  if (logoUrl && !failed) {
    return (
      <span className="app-logo-image-frame" style={{ display: "inline-grid", width: size, height: size, placeItems: "center", overflow: "hidden", borderRadius: 8 }}>
        <img
          src={logoUrl}
          alt={siteName}
          style={{ width: "100%", height: "100%", objectFit: "cover" }}
          onError={() => setFailed(true)}
        />
      </span>
    );
  }

  return (
    <span style={{
      display: "inline-grid",
      width: size,
      height: size,
      placeItems: "center",
      borderRadius: 8,
      background: "radial-gradient(circle at 30% 26%, #ffffff 0 8%, transparent 9%), linear-gradient(135deg, #2f6bff, #8b5cff 54%, #28d6ff)",
      boxShadow: "0 0 26px rgba(40,214,255,0.35), inset 0 1px 0 rgba(255,255,255,0.42)",
      color: "#fff",
    }}>
      <Sparkles className="size-4" />
    </span>
  );
}
