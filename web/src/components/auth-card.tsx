"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ArrowRight, LoaderCircle, LockKeyhole, MailCheck, Sparkles } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
import { cn } from "@/lib/utils";

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
  const authTitle =
    mode === "register" ? "Create your account" : "Log in to Image Studio";
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
  const inputClass =
    "h-12 rounded-lg border-white/15 bg-[#182130] text-white shadow-none placeholder:text-[#7f8a9d] focus-visible:ring-[rgba(80,183,255,0.18)]";

  return (
    <section
      className={cn(
        "relative w-full overflow-hidden rounded-lg border border-white/15 bg-[#05070c] p-6 shadow-[0_34px_120px_rgba(0,0,0,0.74),0_0_74px_rgba(44,137,255,0.16),inset_0_1px_0_rgba(255,255,255,0.16)] sm:p-8",
        className,
      )}
    >
      <div className="pointer-events-none absolute inset-x-0 top-0 h-28 bg-[radial-gradient(circle_at_30%_0%,rgba(91,214,255,0.16),transparent_18rem),linear-gradient(180deg,rgba(255,255,255,0.065),transparent)]" />
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(255,255,255,0.035),transparent_42%)]" />
      <div className="relative z-10">
      <div className="inline-flex items-center gap-3 text-sm font-bold">
        <AuthBrandMark logoUrl={site.logoUrl} siteName={site.name} compact />
        <span>{site.name || "Image Studio"}</span>
      </div>

      <div className="mt-8">
        <h2 className="text-3xl font-bold leading-tight text-white">
          {authTitle}
        </h2>
        <p className="mt-3 text-sm leading-6 text-[#9ca6b8]">{authSubtitle}</p>
      </div>

      <div className="mt-7 grid gap-4">
        <Field label="邮箱" htmlFor="email">
          <Input
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
            className={inputClass}
          />
        </Field>

        {mode === "register" ? (
          <Field label="用户名" htmlFor="username">
            <Input
              id="username"
              name="username"
              type="text"
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              placeholder="你的名称(选填)"
              className={inputClass}
            />
          </Field>
        ) : null}

        <Field label="密码" htmlFor="password">
          <Input
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
            className={inputClass}
          />
        </Field>

        {mode === "register" ? (
          <>
            <Field label="确认密码" htmlFor="confirm-password">
              <Input
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
                className={inputClass}
              />
            </Field>

            <Field label="验证码" htmlFor="verification-code">
              <div className="flex gap-2">
                <Input
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
                  className={inputClass}
                />
                <Button
                  type="button"
                  variant="outline"
                  className="h-12 min-w-[96px] shrink-0 rounded-lg border-white/15 bg-[#111823] text-[#e9eef7] shadow-none hover:bg-[#182130]"
                  onClick={() => void handleSendCode()}
                  disabled={!canSendCode || formDisabled}
                >
                  {isSendingCode ? (
                    <LoaderCircle className="size-4 animate-spin" />
                  ) : (
                    <MailCheck className="size-4" />
                  )}
                  {codeCooldownLeft > 0 ? `${codeCooldownLeft}s` : "发送"}
                </Button>
              </div>
            </Field>
            {registrationCodeRequired ? (
              <Field label="注册码" htmlFor="register-code">
                <Input
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
                  className={inputClass}
                />
              </Field>
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

        <Button
          className="mt-2 h-12 w-full rounded-lg border border-cyan-200/40 bg-[linear-gradient(135deg,rgba(255,255,255,0.18),rgba(255,255,255,0.05)),linear-gradient(135deg,rgba(63,105,255,0.9),rgba(31,220,255,0.78))] text-white shadow-[0_18px_40px_rgba(41,152,255,0.26)] hover:bg-[linear-gradient(135deg,rgba(255,255,255,0.22),rgba(255,255,255,0.08)),linear-gradient(135deg,rgba(63,105,255,0.95),rgba(31,220,255,0.82))]"
          onClick={() => void submit()}
          disabled={isSubmitting || (visibleTurnstileRequired && !turnstileToken)}
        >
          {isSubmitting ? (
            <LoaderCircle className="size-4 animate-spin" />
          ) : mode === "register" ? (
            <MailCheck className="size-4" />
          ) : (
            <LockKeyhole className="size-4" />
          )}
          {mode === "register" ? "注册并登录" : "登录"}
          <ArrowRight className="size-4" />
        </Button>

        {mode === "login" ? (
          <div className="-mt-1 flex justify-end">
            <Link
              to="/forgot-password"
              className="text-sm font-semibold text-[#77dfff] transition hover:text-[#a7ecff] hover:underline"
            >
              忘记密码？
            </Link>
          </div>
        ) : null}

        <p className="text-center text-sm leading-6 text-[#8d98aa]">
          {mode === "register" ? (
            <>
              已有账号？{" "}
              <button
                type="button"
                className="font-bold text-[#77dfff] hover:underline"
                onClick={() => switchMode("login")}
              >
                返回登录
              </button>
            </>
          ) : (
            <>
              还没有账户？{" "}
              <button
                type="button"
                className="font-bold text-[#77dfff] hover:underline"
                onClick={handleOpenRegister}
              >
                创建账号
              </button>
            </>
          )}
        </p>
      </div>
      </div>
    </section>
  );
}

function Field({
  label,
  htmlFor,
  children,
}: {
  label: string;
  htmlFor: string;
  children: ReactNode;
}) {
  return (
    <div className="grid gap-2">
      <label htmlFor={htmlFor} className="text-sm font-semibold text-[#c6cfdd]">
        {label}
      </label>
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
  const size = compact ? "size-[30px]" : "size-8";

  useEffect(() => {
    setFailed(false);
  }, [logoUrl]);

  if (logoUrl && !failed) {
    return (
      <span className={`app-logo-image-frame inline-grid ${size} place-items-center overflow-hidden rounded-lg`}>
        <img
          src={logoUrl}
          alt={siteName}
          className="size-full object-cover"
          onError={() => setFailed(true)}
        />
      </span>
    );
  }

  return (
    <span className={`inline-grid ${size} place-items-center rounded-lg bg-[linear-gradient(135deg,#2f6bff,#8b5cff_54%,#28d6ff)] shadow-[0_0_26px_rgba(40,214,255,0.35)]`}>
      <Sparkles className="size-4" />
    </span>
  );
}
