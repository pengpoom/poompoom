"use client";

import { useEffect, useState, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ArrowLeft, CheckCircle2, LoaderCircle, MailCheck, ShieldCheck } from "lucide-react";
import { toast } from "sonner";

import { AuthBrandMark } from "@/components/auth-card";
import { TurnstileWidget } from "@/components/turnstile-widget";
import { requestPasswordResetCode, resetBusinessUserPasswordByEmail } from "@/lib/api";
import { usePublicSiteSettings, usePublicTurnstileSettings } from "@/lib/site-settings";

export default function ForgotPasswordPage() {
  const navigate = useNavigate();
  const site = usePublicSiteSettings();
  const turnstile = usePublicTurnstileSettings();
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [turnstileToken, setTurnstileToken] = useState("");
  const [turnstileResetKey, setTurnstileResetKey] = useState(0);
  const [cooldownLeft, setCooldownLeft] = useState(0);
  const [isSendingCode, setIsSendingCode] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [completed, setCompleted] = useState(false);
  const turnstileRequired = Boolean(turnstile.enabled && turnstile.siteKey && turnstile.passwordReset);
  const resetTurnstile = () => {
    setTurnstileToken("");
    setTurnstileResetKey((current) => current + 1);
  };

  useEffect(() => {
    if (cooldownLeft <= 0) {
      return;
    }
    const timer = window.setInterval(() => {
      setCooldownLeft((current) => Math.max(0, current - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [cooldownLeft]);

  const handleSendCode = async () => {
    const normalizedEmail = email.trim();
    if (!normalizedEmail) {
      toast.error("请输入邮箱");
      return;
    }
    if (cooldownLeft > 0 || isSendingCode) {
      return;
    }
    if (turnstileRequired && !turnstileToken) {
      toast.error("请先完成人机验证");
      return;
    }
    setIsSendingCode(true);
    try {
      const result = await requestPasswordResetCode(normalizedEmail, turnstileToken);
      resetTurnstile();
      setCooldownLeft(Math.max(1, result.cooldownSeconds || 60));
      toast.success("如果邮箱已注册，验证码会发送到该邮箱");
    } catch (error) {
      resetTurnstile();
      toast.error(error instanceof Error ? error.message : "发送验证码失败");
    } finally {
      setIsSendingCode(false);
    }
  };

  const handleResetPassword = async () => {
    const normalizedEmail = email.trim();
    const nextPassword = password.trim();
    if (!normalizedEmail || !code.trim() || !nextPassword || !confirmPassword.trim()) {
      toast.error("请输入邮箱、验证码和新密码");
      return;
    }
    if (nextPassword.length < 6) {
      toast.error("新密码至少 6 位");
      return;
    }
    if (nextPassword !== confirmPassword.trim()) {
      toast.error("两次输入的新密码不一致");
      return;
    }
    setIsSubmitting(true);
    try {
      await resetBusinessUserPasswordByEmail({
        email: normalizedEmail,
        code,
        password: nextPassword,
      });
      setCompleted(true);
      toast.success("密码已重置");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "重置密码失败");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="auth-shell">
      <div className="auth-bg-beam left" aria-hidden="true" />
      <div className="auth-bg-beam right" aria-hidden="true" />
      <div className="auth-bg-grid" aria-hidden="true" />

      <header className="auth-top">
        <Link to="/login" className="auth-back">
          <ArrowLeft className="size-4" />
          返回登录
        </Link>
        <div className="auth-brand">
          <AuthBrandMark logoUrl={site.logoUrl} siteName={site.name} />
          <span>{site.name || "Poom Studio"}</span>
        </div>
      </header>

      <main style={{ position: "relative", zIndex: 10, maxWidth: 520, margin: "0 auto", minHeight: "calc(100vh - 96px)", display: "flex", alignItems: "center", padding: "40px 0" }}>
        <section className="auth-card" style={{ width: "100%" }}>
          <div className="auth-card-inner">
            <div className="auth-icon-frame">
              {completed ? <CheckCircle2 className="size-6" /> : <ShieldCheck className="size-6" />}
            </div>
            <h2>{completed ? "密码已重置" : "重置密码"}</h2>
            <p className="auth-sub">
              {completed ? "请使用新密码重新登录。" : "通过注册邮箱接收验证码，然后设置新密码。"}
            </p>

            {completed ? (
              <button
                type="button"
                className="auth-primary"
                style={{ marginTop: 28 }}
                onClick={() => navigate("/login", { replace: true })}
              >
                返回登录
              </button>
            ) : (
              <div className="auth-fields">
                <AuthField label="邮箱" htmlFor="reset-email">
                  <div className="auth-code-row">
                    <input
                      id="reset-email"
                      name="email"
                      type="email"
                      autoComplete="email"
                      value={email}
                      onChange={(event) => setEmail(event.target.value)}
                      placeholder="you@example.com"
                      className="auth-input"
                    />
                    <button
                      type="button"
                      className="auth-code-send"
                      onClick={() => void handleSendCode()}
                      disabled={cooldownLeft > 0 || isSendingCode || isSubmitting}
                    >
                      {isSendingCode ? <LoaderCircle className="size-4 animate-spin" /> : <MailCheck className="size-4" />}
                      {cooldownLeft > 0 ? `${cooldownLeft}s` : "发送"}
                    </button>
                  </div>
                </AuthField>

                <AuthField label="验证码" htmlFor="reset-code">
                  <input
                    id="reset-code"
                    name="code"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    value={code}
                    onChange={(event) => setCode(event.target.value)}
                    placeholder="邮箱验证码"
                    className="auth-input"
                  />
                </AuthField>

                <AuthField label="新密码" htmlFor="reset-password">
                  <input
                    id="reset-password"
                    name="password"
                    type="password"
                    autoComplete="new-password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    placeholder="至少 6 位"
                    className="auth-input"
                  />
                </AuthField>

                <AuthField label="确认新密码" htmlFor="reset-confirm-password">
                  <input
                    id="reset-confirm-password"
                    name="confirm-password"
                    type="password"
                    autoComplete="new-password"
                    value={confirmPassword}
                    onChange={(event) => setConfirmPassword(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        void handleResetPassword();
                      }
                    }}
                    placeholder="请再次输入新密码"
                    className="auth-input"
                  />
                </AuthField>

                <TurnstileWidget
                  enabled={turnstileRequired}
                  siteKey={turnstile.siteKey}
                  action="password-reset"
                  disabled={isSendingCode || isSubmitting}
                  resetKey={turnstileResetKey}
                  onTokenChange={setTurnstileToken}
                />

                <button
                  type="button"
                  className="auth-primary"
                  onClick={() => void handleResetPassword()}
                  disabled={isSubmitting || isSendingCode}
                >
                  {isSubmitting ? <LoaderCircle className="size-4 animate-spin" /> : null}
                  重置密码
                </button>
              </div>
            )}
          </div>
        </section>
      </main>
    </div>
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
