"use client";

import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import "react-medium-image-zoom/dist/styles.css";
import {
  ChevronsDown,
  PanelLeftOpen,
} from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";

import { ImageEditModal } from "@/components/image-edit-modal";
import {
  cancelBusinessImageJob,
  fetchAccounts,
  fetchBusinessImageJob,
  fetchBusinessSystemSettings,
  type APIAccessPlatform,
  type Account,
  type BusinessImageJob,
  type ImageQuality,
} from "@/lib/api";
import webConfig from "@/constants/common-env";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import {
  normalizeConversation,
  renameImageConversation,
  saveImageConversation,
  updateImageConversation,
  type ImageConversation,
  type ImageConversationTurn,
  type ImageMode,
} from "@/store/image-conversations";
import { ConversationTurns } from "./components/conversation-turns";
import { EmptyState } from "./components/empty-state";
import { HistorySidebar } from "./components/history-sidebar";
import { PromptComposer } from "./components/prompt-composer";
import { WorkspaceHeader } from "./components/workspace-header";
import { useImageHistory } from "./hooks/use-image-history";
import { useImageSourceInputs } from "./hooks/use-image-source-inputs";
import { useImageSubmit } from "./hooks/use-image-submit";
import { formatImageConversationTitle } from "./title-utils";
import { buildConversationPreviewSource } from "./view-utils";

type ImageAspectRatio = "auto" | "1:1" | "2:3" | "3:2" | "3:4" | "4:3" | "9:16" | "16:9" | "21:9";
type ImageResolutionTier = "auto-free" | "auto-paid" | "sd" | "2k" | "4k";
type ImageResolutionAccess = "free" | "paid";
type ActiveRequestState = {
  conversationId: string;
  turnId: string;
  mode: ImageMode;
  count: number;
  variant: "standard" | "selection-edit";
};
type ImageResolutionPreset = {
  tier: ImageResolutionTier;
  label: string;
  value: string;
  access: ImageResolutionAccess;
};

const imageAspectRatioOptions: Array<{
  label: string;
  value: ImageAspectRatio;
}> = [
  { label: "智能比例", value: "auto" },
  { label: "1:1", value: "1:1" },
  { label: "2:3", value: "2:3" },
  { label: "3:2", value: "3:2" },
  { label: "3:4", value: "3:4" },
  { label: "4:3", value: "4:3" },
  { label: "9:16", value: "9:16" },
  { label: "16:9", value: "16:9" },
];

const imageAutoResolutionPresets: ImageResolutionPreset[] = [
  { tier: "sd", label: "1K", value: "", access: "free" },
  { tier: "2k", label: "2K", value: "", access: "paid" },
  { tier: "4k", label: "4K", value: "", access: "paid" },
];

const imageResolutionPresets: Record<
  Exclude<ImageAspectRatio, "auto">,
  ImageResolutionPreset[]
> = {
  "1:1": [
    { tier: "sd", label: "1K", value: "1248x1248", access: "free" },
    { tier: "2k", label: "2K", value: "2048x2048", access: "paid" },
    {
      tier: "4k",
      label: "4K",
      value: "2880x2880",
      access: "paid",
    },
  ],
  "2:3": [
    { tier: "sd", label: "1K", value: "1024x1536", access: "free" },
    { tier: "2k", label: "2K", value: "1440x2160", access: "paid" },
    { tier: "4k", label: "4K", value: "2304x3456", access: "paid" },
  ],
  "3:2": [
    { tier: "sd", label: "1K", value: "1536x1024", access: "free" },
    { tier: "2k", label: "2K", value: "2160x1440", access: "paid" },
    { tier: "4k", label: "4K", value: "3456x2304", access: "paid" },
  ],
  "3:4": [
    { tier: "sd", label: "1K", value: "1072x1440", access: "free" },
    { tier: "2k", label: "2K", value: "1536x2048", access: "paid" },
    { tier: "4k", label: "4K", value: "2448x3264", access: "paid" },
  ],
  "4:3": [
    { tier: "sd", label: "1K", value: "1440x1072", access: "free" },
    { tier: "2k", label: "2K", value: "2048x1536", access: "paid" },
    { tier: "4k", label: "4K", value: "3264x2448", access: "paid" },
  ],
  "16:9": [
    { tier: "sd", label: "1K", value: "1664x928", access: "free" },
    { tier: "2k", label: "2K", value: "2560x1440", access: "paid" },
    { tier: "4k", label: "4K", value: "3840x2160", access: "paid" },
  ],
  "21:9": [
    { tier: "sd", label: "1K", value: "1904x816", access: "free" },
    { tier: "2k", label: "2K", value: "3360x1440", access: "paid" },
    { tier: "4k", label: "4K", value: "3808x1632", access: "paid" },
  ],
  "9:16": [
    { tier: "sd", label: "1K", value: "928x1664", access: "free" },
    { tier: "2k", label: "2K", value: "1440x2560", access: "paid" },
    { tier: "4k", label: "4K", value: "2160x3840", access: "paid" },
  ],
};

const modeOptions: Array<{
  label: string;
  value: ImageMode;
  description: string;
}> = [
  {
    label: "生成",
    value: "generate",
    description: "提示词生成新图，也可上传参考图辅助生成",
  },
  { label: "编辑", value: "edit", description: "上传图像后局部或整体改图" },
];
const imageQualityOptions: Array<{
  label: string;
  value: ImageQuality;
  description: string;
}> = [
  { label: "Low", value: "low", description: "低质量，速度更快，适合草稿测试" },
  {
    label: "Medium",
    value: "medium",
    description: "均衡质量与速度，适合日常生成",
  },
  {
    label: "High",
    value: "high",
    description: "高质量，耗时更长，适合最终出图",
  },
];
const providerPlatformOptions: Array<{
  label: string;
  value: APIAccessPlatform;
}> = [
  { label: "gpt-image", value: "gpt-image" },
  { label: "gemini-banana", value: "gemini-banana" },
];

const modeLabelMap: Record<ImageMode, string> = {
  generate: "生成",
  edit: "编辑",
};

const inspirationExamples: Array<{
  id: string;
  title: string;
  prompt: string;
  hint: string;
  count: number;
  tone: string;
}> = [
  {
    id: "stellar-poster",
    title: "卡芙卡轮廓宇宙海报",
    prompt:
      "请根据【主题：崩坏星穹铁道，角色卡芙卡】自动生成一张高审美的“轮廓宇宙 / 收藏版叙事海报”风格作品。不要将画面局限于固定器物或常见容器，不要优先默认瓶子、沙漏、玻璃罩、怀表之类的常规载体，而是由 AI 根据主题自行判断并选择一个最契合、最有象征意义、轮廓最强、最适合承载完整叙事世界的主轮廓载体。这个主轮廓可以是器物、建筑、门、塔、拱门、穹顶、楼梯井、长廊、雕像、侧脸、眼睛、手掌、头骨、羽翼、面具、镜面、王座、圆环、裂缝、光幕、阴影、几何结构、空间切面、舞台框景、抽象符号或其他更有创意与主题代表性的视觉轮廓，要求合理布局。优先选择最能放大主题气质、最能形成强烈视觉记忆点、最能体现史诗感、神秘感、诗意感或设计感的轮廓，而不是最安全、最普通、最常见的容器。画面的核心不是简单把世界装进某个物体里，而是让完整的主题世界自然生长在这个主轮廓之中、之内、之上、之边界里或与其结构融为一体，形成一种“主题宇宙依附于一个象征性轮廓展开”的高级叙事效果。主轮廓必须清晰、优雅、有辨识度，并在整体构图中占据核心地位。轮廓内部或边界中需要自动生成与主题强绑定的完整叙事世界，内容应当丰富、饱满、层次清晰，包括最能代表主题的标志性场景、核心建筑或空间结构、象征符号与隐喻元素、角色关系或文明痕迹、远景中景近景的空间递进、具有命运感和情绪张力的氛围层次，以及门、台阶、桥梁、水面、烟雾、路径、光源、遗迹、机械结构、自然景观、抽象形态、生物或道具等叙事细节。所有元素必须统一、自然、有主次、有层级地融合，像一个完整世界真实孕育在这个轮廓结构之中，而不是简单拼贴、裁切填充、素材堆叠或模板化背景。整体构图需要具有强烈的收藏版海报气质与高级设计感，大结构稳定，主轮廓强烈明确，内部世界具有纵深、秩序和呼吸感，细节丰富但不拥挤，内容丰满但不杂乱，可以适度加入小比例人物剪影、远处建筑、光柱、门洞、桥、阶梯、回廊、倒影、天光或远景结构来增强尺度感、故事感与史诗感。整体画面要安静、宏大、凝练、富有余味，不要平均铺满，不要廉价热闹，不要无重点堆砌。风格融合收藏版电影海报构图、高级叙事型视觉设计、梦幻水彩质感与纸张印刷品气质，强调纸张颗粒感、边缘飞白、水彩刷痕、轻微晕染、空气透视、柔和雾化、局部体积光、光雾穿透、大面积留白与克制版式，让画面看起来像设计师完成的高端收藏版视觉作品，而不是普通 AI 跑图。整体气质要高级、诗意、宏大、神圣、怀旧、安静、具有传说感和叙事感。色彩由 AI 根据主题自动判断并匹配最合适的高级配色方案，但必须保持统一、克制、耐看、低饱和、高级，不要杂乱高饱和，不要廉价霓虹感，不要塑料数码感。配色可以围绕黑金灰、冷蓝灰、雾白灰、褐红米白、暗铜、旧纸色、深海蓝、暮色紫、银灰等体系自由变化，但必须始终服务主题，并保持海报级审美与整体和谐。最终要求：第一眼有强烈的主题识别度和轮廓记忆点，第二眼有完整丰富的叙事世界，第三眼仍有细节和余味。轮廓选择必须具有创意和主题匹配度，尽量避免重复、保守、常见的容器套路，优先选择更有象征性、更有空间感、更有设计潜力的轮廓形式。不要普通背景拼接，不要生硬裁切，不要模板化奇幻素材，不要游戏宣传图感，不要过度卡通化，不要过度写实导致失去艺术感，不要形式大于内容。如果合适，可以自然加入低调克制的标题、编号、签名或落款，让它更像收藏版海报设计的一部分，但不要喧宾夺主。",
    hint: "适合高审美叙事海报、角色宇宙主题视觉、收藏版概念海报。",
    count: 1,
    tone: "from-[#17131f] via-[#4c2d45] to-[#b79b8b]",
  },
  {
    id: "qinghua-museum-infographic",
    title: "青花瓷博物馆图鉴",
    prompt:
      "请根据“青花瓷”自动生成一张“博物馆图鉴式中文拆解信息图”。要求整张图兼具真实写实主视觉、结构拆解、中文标注、材质说明、纹样寓意、色彩含义和核心特征总结。你需要根据主题自动判断最合适的主体对象、服饰体系、器物结构、时代风格、关键部件、材质工艺、颜色方案与版式结构，用户无需再提供其他信息。整体风格应为：国家博物馆展板、历史服饰图鉴、文博专题信息图，而不是普通海报、古风写真、电商详情页或动漫插画。背景采用米白、绢纸白、浅茶色等纸张质感，整体高级、克制、专业、可收藏。版式固定为：顶部：中文主标题 + 副标题 + 导语；左侧：结构拆解区，中文引线标注关键部件，并配局部特写；右上：材质 / 工艺 / 质感区，展示真实纹理小样并附说明；右中：纹样 / 色彩 / 寓意区，展示主色板、纹样样本和文化解释；底部：穿着顺序 / 构成流程图 + 核心特征总结。若主题适合人物展示，则以真实人物全身站姿为中央主体；若更适合器物或单体结构，则改为中心主体拆解图，但整体仍保持完整中文信息图形式。所有文字必须为简体中文，清晰、规整、可读，不要乱码、错字、英文或拼音。重点突出真实结构、材质差异、文化说明与图鉴气质。避免：海报感、影楼感、电商感、动漫感、cosplay感、乱标注、错结构、糊字、假材质、过度装饰。",
    hint: "适合文博专题、器物拆解、中文信息图和展板式视觉。",
    count: 1,
    tone: "from-[#0d2f5f] via-[#3a6ea5] to-[#e7dcc4]",
  },
  {
    id: "editorial-fashion",
    title: "周芷若联动宣传图",
    prompt:
      "《倚天屠龙记》周芷若的维秘联动活动宣传图，人物占画面 80% 以上，周芷若在古风古城城墙上，优雅侧身回眸姿态，突出古典美人身姿曲线， 穿着维秘联动款：融合古风元素的蕾丝吊带裙，搭配精致吊带丝袜（黑色或淡青色，带有轻微古风刺绣），丝袜包裹修长双腿，整体造型唯美古典， 高品质真人级 3D 古风游戏截图风格，电影级光影，周芷若清丽绝俗、长发微散，眼神柔美回眸，轻纱飘逸， 背景为夜晚古城墙，青砖城垛、灯笼照明、月光洒落，古建筑灯火点点，氛围梦幻唯美， 高细节，8K 品质，精致渲染，真实丝袜质感，电影级构图，光影细腻，古典武侠风",
    hint: "适合古风角色联动、游戏活动主视觉、电影感人物宣传图。",
    count: 1,
    tone: "from-zinc-900 via-rose-800 to-amber-500",
  },
  {
    id: "forza-horizon-shenzhen",
    title: "地平线 8 深圳实机图",
    prompt:
      "创作一张图片为《极限竞速 地平线 8》的游戏实机截图，游戏背景设为中国，背景城市为深圳，时间设定为 2028 年。画面需要体现真实次世代开放世界赛车游戏的实机演出效果，包含具有深圳辨识度的城市天际线、现代高楼、道路环境、灯光氛围与速度感。构图中在合适位置放置《极限竞速 地平线 8》的 logo 及宣传文案，整体像官方概念宣传截图而不是普通海报。要求 8K 超高清，电影级光影，真实车辆材质、反射、路面细节与空气透视，画面高级、震撼、写实。",
    hint: "适合游戏主视觉、次世代赛车截图、城市宣传感概念图。",
    count: 1,
    tone: "from-slate-950 via-cyan-900 to-orange-500",
  },
];

function formatConversationTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function getImageRemaining(account: Account) {
  const limit = account.limits_progress?.find(
    (item) => item.feature_name === "image_gen",
  );
  if (typeof limit?.remaining === "number") {
    return Math.max(0, limit.remaining);
  }
  return Math.max(0, account.quota);
}

function isImageAccountUsable(account: Account, allowDisabled: boolean) {
  const disabled = Boolean(account.disabled) || account.status === "禁用";
  return (
    (!disabled || allowDisabled) &&
    account.status !== "异常" &&
    account.status !== "限流" &&
    getImageRemaining(account) > 0
  );
}

function isBusinessProxyMode() {
  return webConfig.backendMode === "business_proxy";
}

const COMPOSER_SAFE_BOTTOM_OFFSET = 188;

function hasAvailablePaidImageAccount(
  accounts: Account[],
  allowDisabled: boolean,
) {
  return accounts.some(
    (account) =>
      isImageAccountUsable(account, allowDisabled) &&
      (account.type === "Plus" ||
        account.type === "Pro" ||
        account.type === "Team"),
  );
}

async function normalizeConversationHistory(items: ImageConversation[]) {
  return items.map((item) => normalizeConversation(item));
}

function makeId() {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function formatProcessingDuration(totalSeconds: number) {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes <= 0) {
    return `${seconds}s`;
  }
  return `${minutes}m ${String(seconds).padStart(2, "0")}s`;
}

function buildWaitingDots(totalSeconds: number) {
  return ".".repeat((totalSeconds % 3) + 1);
}

type ProcessingJobTarget = {
  jobId: string;
  conversationId: string;
};

function isProcessingTurn(turn: ImageConversationTurn) {
  if (turn.status === "cancelled") {
    return false;
  }
  return (
    turn.status === "queued" ||
    turn.status === "running" ||
    turn.status === "generating" ||
    Boolean(turn.cancelRequested) ||
    turn.images.some((image) => image.status === "loading")
  );
}

function isProcessingConversation(conversation: ImageConversation) {
  return (conversation.turns || []).some(isProcessingTurn);
}

function hasLegacyProcessingTurn(conversation: ImageConversation) {
  return (conversation.turns || []).some(
    (turn) => isProcessingTurn(turn) && !turn.jobId,
  );
}

function collectProcessingJobTargets(
  conversations: ImageConversation[],
): ProcessingJobTarget[] {
  const byJobId = new Map<string, ProcessingJobTarget>();
  for (const conversation of conversations) {
    for (const turn of conversation.turns || []) {
      if (!isProcessingTurn(turn) || !turn.jobId) {
        continue;
      }
      byJobId.set(turn.jobId, {
        jobId: turn.jobId,
        conversationId: conversation.id,
      });
    }
  }
  return [...byJobId.values()].sort((a, b) =>
    a.jobId.localeCompare(b.jobId),
  );
}

function buildBusinessJobSignature(job: BusinessImageJob) {
  return [
    job.status,
    job.stage || "",
    job.errorCode || "",
    job.errorMessage || "",
    job.actualCount,
    job.storageBytes,
    job.updatedAt,
    job.finishedAt || "",
  ].join("|");
}

function isTerminalBusinessJob(job: BusinessImageJob) {
  const status = String(job.status || "").trim().toLowerCase();
  return (
    status === "succeeded" ||
    status === "success" ||
    status === "failed" ||
    status === "error" ||
    status === "cancelled" ||
    status === "canceled"
  );
}

function buildProcessingStatus(
  mode: ImageMode,
  elapsedSeconds: number,
  count: number,
  variant: ActiveRequestState["variant"] = "standard",
) {
  if (mode === "generate") {
    if (elapsedSeconds < 4) {
      return {
        title: "正在提交生成请求",
        detail: `已进入图像生成队列，本次目标 ${count} 张`,
      };
    }
    if (elapsedSeconds < 12) {
      return {
        title: "正在排队创建画面",
        detail: "模型正在准备构图与风格细节",
      };
    }
    return {
      title: "模型正在生成图片",
      detail: "通常需要 1 到 5 分钟，请保持页面开启",
    };
  }

  if (mode === "edit") {
    if (elapsedSeconds < 4) {
      return {
        title:
          variant === "selection-edit"
            ? "正在提交选区编辑"
            : "正在提交编辑请求",
        detail: "请求已发送，正在准备处理素材",
      };
    }
    if (elapsedSeconds < 12) {
      return {
        title:
          variant === "selection-edit"
            ? "正在上传源图和选区"
            : "正在上传编辑素材",
        detail: "素材上传完成后会立即进入改图阶段",
      };
    }
    return {
      title:
        variant === "selection-edit"
          ? "模型正在按选区修改图片"
          : "模型正在编辑图片",
      detail: "通常需要 1 到 5 分钟，请保持页面开启",
    };
  }

  return {
    title: "模型正在编辑图片",
    detail: "通常需要 1 到 5 分钟，请保持页面开启",
  };
}

export default function ImagePage() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const didLoadQuotaRef = useRef(false);
  const mountedRef = useRef(true);
  const draftSelectionRef = useRef(false);
  const uploadInputRef = useRef<HTMLInputElement | null>(null);
  const maskInputRef = useRef<HTMLInputElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const resultsViewportRef = useRef<HTMLDivElement | null>(null);
  const resultsContentRef = useRef<HTMLDivElement | null>(null);
  const bottomAnchorRef = useRef<HTMLDivElement | null>(null);
  const bottomScrollLockUntilRef = useRef(0);
  const isNearBottomRef = useRef(true);
  const previousSelectedConversationIdRef = useRef<string | null>(null);
  const previousTurnCountRef = useRef(0);
  const previousLastTurnKeyRef = useRef("");
  const jobPollingSignaturesRef = useRef<Map<string, string>>(new Map());

  const [mode, setMode] = useState<ImageMode>("generate");
  const [imagePrompt, setImagePrompt] = useState("");
  const [imageCount, setImageCount] = useState("1");
  const [imageAspectRatio, setImageAspectRatio] =
    useState<ImageAspectRatio>("1:1");
  const [imageResolutionTier, setImageResolutionTier] =
    useState<ImageResolutionTier>("sd");
  const [imageQuality, setImageQuality] = useState<ImageQuality>("high");
  const [providerPlatform, setProviderPlatform] =
    useState<APIAccessPlatform>("gpt-image");
  const [selectionEditorProviderPlatform, setSelectionEditorProviderPlatform] =
    useState<APIAccessPlatform>("gpt-image");
  const [composerResetKey, setComposerResetKey] = useState(0);
  const [historyCollapsed, setHistoryCollapsed] = useState(false);
  const [isDesktopLayout, setIsDesktopLayout] = useState(() =>
    typeof window !== "undefined"
      ? window.matchMedia("(min-width: 1024px)").matches
      : false,
  );
  const [availableAccounts, setAvailableAccounts] = useState<Account[]>([]);
  const [submitElapsedSeconds, setSubmitElapsedSeconds] = useState(0);
  const [showScrollToBottom, setShowScrollToBottom] = useState(false);
  const [isMobileComposerCollapsed, setIsMobileComposerCollapsed] =
    useState(true);
  const emptyActiveConversationIds = useMemo(() => new Set<string>(), []);

  const {
    conversations,
    selectedConversationId,
    isLoadingHistory,
    setConversations,
    setSelectedConversationId,
    focusConversation,
    openDraftConversation,
    refreshHistory,
    refreshConversation,
    handleCreateDraft,
    handleDeleteConversation,
    handleClearHistory,
  } = useImageHistory({
    normalizeHistory: normalizeConversationHistory,
    mountedRef,
    draftSelectionRef,
    processingConversationIds: emptyActiveConversationIds,
    preferredProcessingConversationId: null,
  });
  const {
    sourceImages,
    setSourceImages,
    editorTarget,
    appendFiles,
    handlePromptPaste,
    removeSourceImage,
    openSelectionEditor,
    openSourceSelectionEditor,
    closeSelectionEditor,
  } = useImageSourceInputs({
    mode,
    selectedConversationId,
    makeId,
  });
  useEffect(() => {
    if (!editorTarget) {
      return;
    }
    setSelectionEditorProviderPlatform(
      editorTarget.providerPlatform ?? providerPlatform,
    );
  }, [editorTarget, providerPlatform]);
  const displayedConversations = conversations;
  const processingConversationIds = useMemo(
    () =>
      new Set(
        displayedConversations
          .filter((conversation) => isProcessingConversation(conversation))
          .map((conversation) => conversation.id),
      ),
    [displayedConversations],
  );
  const processingJobTargets = useMemo(
    () => collectProcessingJobTargets(displayedConversations),
    [displayedConversations],
  );
  const processingJobTargetKey = useMemo(
    () =>
      processingJobTargets
        .map((item) => `${item.jobId}:${item.conversationId}`)
        .join("|"),
    [processingJobTargets],
  );
  const legacyProcessingConversationKey = useMemo(
    () =>
      displayedConversations
        .filter(hasLegacyProcessingTurn)
        .map((conversation) => conversation.id)
        .sort()
        .join("|"),
    [displayedConversations],
  );
  const selectedConversation = useMemo(
    () =>
      displayedConversations.find((item) => item.id === selectedConversationId) ??
      null,
    [displayedConversations, selectedConversationId],
  );
  const currentImageView = useMemo<"history" | "workspace">(
    () => (pathname.endsWith("/workspace") ? "workspace" : "history"),
    [pathname],
  );
  const isStandaloneHistory =
    !isDesktopLayout && currentImageView === "history";
  const isStandaloneWorkspace =
    !isDesktopLayout && currentImageView === "workspace";
  const selectedConversationTurns = useMemo(
    () => selectedConversation?.turns ?? [],
    [selectedConversation],
  );
  const selectedConversationLastTurn = useMemo(
    () =>
      selectedConversationTurns[selectedConversationTurns.length - 1] ?? null,
    [selectedConversationTurns],
  );
  const selectedConversationLastTurnKey = useMemo(() => {
    if (!selectedConversationLastTurn) {
      return "";
    }
    const imageKey = selectedConversationLastTurn.images
      .map(
        (image) =>
          `${image.id}:${image.status ?? "loading"}:${image.error ?? ""}`,
      )
      .join("|");
    return `${selectedConversationLastTurn.id}:${selectedConversationLastTurn.status}:${imageKey}`;
  }, [selectedConversationLastTurn]);
  const selectedConversationProcessingTurn = useMemo(() => {
    if (!selectedConversation) {
      return null;
    }
    return [...(selectedConversation.turns || [])]
      .reverse()
      .find((turn) =>
        turn.status === "queued" ||
        turn.status === "running" ||
        turn.status === "generating",
      ) ?? null;
  }, [selectedConversation]);
  const selectedConversationProcessingStartedAt = useMemo(() => {
    const processingTurn = selectedConversationProcessingTurn;
    if (!processingTurn) {
      return null;
    }
    const timestamp = new Date(
      processingTurn.startedAt || processingTurn.createdAt,
    ).getTime();
    return Number.isNaN(timestamp) ? null : timestamp;
  }, [selectedConversationProcessingTurn]);
  const activeRequest = useMemo<ActiveRequestState | null>(
    () => {
      if (!selectedConversation || !selectedConversationProcessingTurn) {
        return null;
      }
      return {
        conversationId: selectedConversation.id,
        turnId: selectedConversationProcessingTurn.id,
        mode: selectedConversationProcessingTurn.mode,
        count: selectedConversationProcessingTurn.count,
        variant:
          selectedConversationProcessingTurn.mode === "edit" &&
          selectedConversationProcessingTurn.sourceImages?.some(
            (source) => source.role === "mask",
          )
            ? "selection-edit"
            : "standard",
      };
    },
    [selectedConversation, selectedConversationProcessingTurn],
  );
  const activeRequestStartedAt = selectedConversationProcessingStartedAt;

  const parsedCount = useMemo(
    () => Math.max(1, Math.min(8, Number(imageCount) || 1)),
    [imageCount],
  );
  const hasAvailablePaidAccount = useMemo(
    () =>
      isBusinessProxyMode() ||
      hasAvailablePaidImageAccount(availableAccounts, false),
    [availableAccounts],
  );
  const currentResolutionPresets = useMemo(
    () =>
      imageAspectRatio === "auto"
        ? imageAutoResolutionPresets
        : imageResolutionPresets[imageAspectRatio],
    [imageAspectRatio],
  );
  const selectedResolutionPreset = useMemo(
    () =>
      currentResolutionPresets.find(
        (item) => item.tier === imageResolutionTier,
      ) ?? currentResolutionPresets[0],
    [currentResolutionPresets, imageResolutionTier],
  );
  const imageQualityDisabledReason = "当前 API 接入支持质量参数。";
  const isImageQualityEnabled = true;
  const imageResolutionTierOptions = useMemo(
    () =>
      currentResolutionPresets.map((item) => ({
        label: item.label,
        value: item.tier,
        disabled: item.access === "paid" && !hasAvailablePaidAccount,
      })),
    [currentResolutionPresets, hasAvailablePaidAccount],
  );
  const imageResolutionTierLabel = useMemo(
    () =>
      imageResolutionTierOptions.find(
        (item) => item.value === imageResolutionTier && !item.disabled,
      )?.label ??
      imageResolutionTierOptions.find((item) => !item.disabled)?.label ??
      "",
    [imageResolutionTier, imageResolutionTierOptions],
  );
  const imageSize = useMemo(
    () =>
      imageAspectRatio === "auto"
        ? ""
        :
      currentResolutionPresets.find(
        (item) =>
          item.tier === imageResolutionTier &&
          (hasAvailablePaidAccount || item.access === "free"),
      )?.value ??
      currentResolutionPresets.find(
        (item) => hasAvailablePaidAccount || item.access === "free",
      )?.value ??
      currentResolutionPresets[0].value,
    [
      currentResolutionPresets,
      hasAvailablePaidAccount,
      imageAspectRatio,
      imageResolutionTier,
    ],
  );
  const imageResolutionAccess = useMemo<ImageResolutionAccess>(
    () => selectedResolutionPreset?.access ?? "free",
    [selectedResolutionPreset],
  );
  const imageSizeHint = useMemo(
    () =>
      mode === "edit" ? (
        <>
          <div>
            <span className="font-semibold text-stone-800">编辑输出尺寸：</span>
            编辑模式会尽量按所选比例和分辨率输出结果，但最终尺寸仍可能受源图比例、遮罩范围和上游模型能力影响。
          </div>
          <div className="mt-2">
            <span className="font-semibold text-stone-800">质量说明：</span>
            输出质量会跟随当前质量档位；如果请求落到 Free legacy
            链路，质量参数可能不会作为正式参数生效。
          </div>
        </>
      ) : (
        <>
          <div>
            <span className="font-semibold text-stone-800">分辨率限制：</span>
            Free 账号当前按约 1.57M 像素总量控制；Paid 账号的图片最长边最高支持
            3840。
          </div>
          <div className="mt-2">
            <span className="font-semibold text-stone-800">账号要求：</span>
            2K 及以上像素档仅 Paid 账号可用，包括 Team / Plus / Pro。
          </div>
          <div className="mt-2">
            <span className="font-semibold text-stone-800">Auto 模式补充：</span>
            当比例切到 Auto 时，当前项目不会强制指定比例和分辨率，请直接在提示词里写明横竖版、画幅比例和目标输出尺寸。`Free / Paid` 只决定调度时优先使用哪类图片账号，不会把固定尺寸写进上游请求。
          </div>
        </>
      ),
    [mode],
  );
  const imageSources = useMemo(
    () => sourceImages.filter((item) => item.role === "image"),
    [sourceImages],
  );
  const processingStatus = useMemo(
    () =>
      activeRequest
        ? buildProcessingStatus(
            activeRequest.mode,
            submitElapsedSeconds,
            activeRequest.count,
            activeRequest.variant,
          )
        : null,
    [activeRequest, submitElapsedSeconds],
  );
  const waitingDots = useMemo(
    () => buildWaitingDots(submitElapsedSeconds),
    [submitElapsedSeconds],
  );

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    const media = window.matchMedia("(min-width: 1024px)");
    const updateLayout = (matches: boolean) => {
      setIsDesktopLayout(matches);
    };

    updateLayout(media.matches);
    const handleChange = (event: MediaQueryListEvent) =>
      updateLayout(event.matches);
    if (typeof media.addEventListener === "function") {
      media.addEventListener("change", handleChange);
      return () => media.removeEventListener("change", handleChange);
    }

    media.addListener(handleChange);
    return () => media.removeListener(handleChange);
  }, []);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      void refreshHistory({
        normalize: true,
        withLoading: conversations.length === 0,
      });
    });

    return () => {
      window.cancelAnimationFrame(frame);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!isBusinessProxyMode() || !processingJobTargetKey) {
      jobPollingSignaturesRef.current.clear();
      return;
    }

    let disposed = false;
    const targets = processingJobTargetKey
      .split("|")
      .map((item) => {
        const [jobId, conversationId] = item.split(":");
        return { jobId, conversationId };
      })
      .filter((item) => item.jobId && item.conversationId);
    const activeJobIds = new Set(targets.map((item) => item.jobId));
    for (const jobId of jobPollingSignaturesRef.current.keys()) {
      if (!activeJobIds.has(jobId)) {
        jobPollingSignaturesRef.current.delete(jobId);
      }
    }

    const pollProcessingJobs = async () => {
      const refreshedConversationIds = new Set<string>();
      await Promise.all(
        targets.map(async ({ jobId, conversationId }) => {
          try {
            const payload = await fetchBusinessImageJob(jobId);
            if (disposed) {
              return;
            }
            const job = payload.item;
            const signature = buildBusinessJobSignature(job);
            const previousSignature = jobPollingSignaturesRef.current.get(jobId);
            const statusChanged = previousSignature !== signature;
            jobPollingSignaturesRef.current.set(jobId, signature);

            if (!statusChanged && !isTerminalBusinessJob(job)) {
              return;
            }

            const nextConversationId =
              job.conversationId || conversationId;
            if (refreshedConversationIds.has(nextConversationId)) {
              return;
            }
            refreshedConversationIds.add(nextConversationId);
            const refreshed = await refreshConversation(nextConversationId, { silent: true });
            if (refreshed && isTerminalBusinessJob(job)) {
              void refreshHistory({ normalize: true, silent: true });
            }
          } catch {
            // 单个 job 轮询失败不阻断其他任务；下一轮继续尝试。
          }
        }),
      );
    };

    void pollProcessingJobs();
    const timer = window.setInterval(() => {
      if (!disposed) {
        void pollProcessingJobs();
      }
    }, 1200);

    return () => {
      disposed = true;
      window.clearInterval(timer);
    };
  }, [processingJobTargetKey, refreshConversation, refreshHistory]);

  useEffect(() => {
    if (!isBusinessProxyMode() || !legacyProcessingConversationKey) {
      return;
    }

    let disposed = false;
    const conversationIds = legacyProcessingConversationKey
      .split("|")
      .filter(Boolean);
    const refreshProcessingConversations = () => {
      for (const conversationId of conversationIds) {
        void refreshConversation(conversationId, { silent: true });
      }
    };

    refreshProcessingConversations();
    const timer = window.setInterval(() => {
      if (!disposed) {
        refreshProcessingConversations();
      }
    }, 2500);

    return () => {
      disposed = true;
      window.clearInterval(timer);
    };
  }, [legacyProcessingConversationKey, refreshConversation]);

  useEffect(() => {
    const loadQuota = async () => {
      try {
        if (isBusinessProxyMode()) {
          try {
            const settingsData = await fetchBusinessSystemSettings();
            const generation = settingsData.settings.generation;
            setProviderPlatform(generation.defaultPlatform);
            setImageQuality(generation.defaultQuality);
            setImageCount(String(Math.max(1, Math.min(8, generation.defaultCount || 1))));
          } catch {
            // 系统设置读取失败不阻塞工作台基础加载。
          }
          return;
        }
        const accountsData = await fetchAccounts();
        setAvailableAccounts(accountsData.items);
      } catch {
        setAvailableAccounts([]);
      }
    };

    if (didLoadQuotaRef.current) {
      return;
    }
    didLoadQuotaRef.current = true;
    void loadQuota();
  }, []);

  useEffect(() => {
    const selectedPreset = currentResolutionPresets.find(
      (item) => item.tier === imageResolutionTier,
    );
    if (
      selectedPreset &&
      (hasAvailablePaidAccount || selectedPreset.access === "free")
    ) {
      return;
    }
    const nextPreset = currentResolutionPresets.find(
      (item) => hasAvailablePaidAccount || item.access === "free",
    );
    if (nextPreset && nextPreset.tier !== imageResolutionTier) {
      setImageResolutionTier(nextPreset.tier);
    }
  }, [currentResolutionPresets, hasAvailablePaidAccount, imageResolutionTier]);

  useEffect(() => {
    if (!isImageQualityEnabled && imageQuality !== "high") {
      setImageQuality("high");
    }
  }, [imageQuality, isImageQualityEnabled]);

  const scrollToBottom = useCallback(
    (behavior: ScrollBehavior = "smooth") => {
      const anchor = bottomAnchorRef.current;
      if (isStandaloneWorkspace) {
        const scrollTarget = document.scrollingElement;
        if (!scrollTarget) {
          return;
        }

        window.scrollTo({
          top: scrollTarget.scrollHeight,
          behavior,
        });
        anchor?.scrollIntoView({
          block: "end",
          inline: "nearest",
          behavior,
        });
        return;
      }

      const viewport = resultsViewportRef.current;
      if (!viewport) {
        return;
      }

      viewport.scrollTo({
        top: viewport.scrollHeight - viewport.clientHeight,
        behavior,
      });
    },
    [isStandaloneWorkspace],
  );

  const resetWorkspaceScrollTop = useCallback(() => {
    bottomScrollLockUntilRef.current = 0;
    isNearBottomRef.current = true;
    setShowScrollToBottom(false);

    const scrollTarget = document.scrollingElement;
    if (scrollTarget) {
      scrollTarget.scrollTop = 0;
    }
    window.scrollTo({ top: 0, behavior: "auto" });
    resultsViewportRef.current?.scrollTo({ top: 0, behavior: "auto" });
  }, []);

  const scheduleScrollToBottom = useCallback(
    (behavior: ScrollBehavior = "auto", lockMs = 0) => {
      if (lockMs > 0) {
        bottomScrollLockUntilRef.current = Math.max(
          bottomScrollLockUntilRef.current,
          Date.now() + lockMs,
        );
      }

      const frames: number[] = [];
      const timers: number[] = [];
      const run = (nextBehavior: ScrollBehavior) => {
        scrollToBottom(nextBehavior);
      };

      frames.push(
        window.requestAnimationFrame(() => {
          run(behavior);
          frames.push(window.requestAnimationFrame(() => run(behavior)));
        }),
      );
      for (const delay of [120, 320, 760]) {
        timers.push(window.setTimeout(() => run("auto"), delay));
      }

      return () => {
        frames.forEach((frame) => window.cancelAnimationFrame(frame));
        timers.forEach((timer) => window.clearTimeout(timer));
      };
    },
    [scrollToBottom],
  );

  useEffect(() => {
    const content = resultsContentRef.current;
    if (!content || typeof ResizeObserver === "undefined") {
      return;
    }

    let frame: number | null = null;
    const observer = new ResizeObserver(() => {
      if (Date.now() > bottomScrollLockUntilRef.current) {
        return;
      }
      if (frame !== null) {
        window.cancelAnimationFrame(frame);
      }
      frame = window.requestAnimationFrame(() => {
        scrollToBottom("auto");
      });
    });

    observer.observe(content);
    return () => {
      observer.disconnect();
      if (frame !== null) {
        window.cancelAnimationFrame(frame);
      }
    };
  }, [scrollToBottom, selectedConversationId]);

  useEffect(() => {
    if (isStandaloneWorkspace) {
      const updateScrollState = () => {
        const scrollTarget = document.scrollingElement;
        if (!scrollTarget) {
          return;
        }
        const scrollTop = window.scrollY || scrollTarget.scrollTop;
        const viewportHeight = window.innerHeight;
        const hiddenHeight =
          scrollTarget.scrollHeight - viewportHeight - scrollTop;
        const hasOverflow = scrollTarget.scrollHeight > viewportHeight + 24;
        const nearBottom = hiddenHeight <= 96;
        isNearBottomRef.current = nearBottom;
        setShowScrollToBottom(hasOverflow && !nearBottom);
      };

      updateScrollState();
      window.addEventListener("scroll", updateScrollState, { passive: true });
      window.addEventListener("resize", updateScrollState);

      return () => {
        window.removeEventListener("scroll", updateScrollState);
        window.removeEventListener("resize", updateScrollState);
      };
    }

    const viewport = resultsViewportRef.current;
    if (!viewport) {
      return;
    }

    const updateScrollState = () => {
      const hiddenHeight =
        viewport.scrollHeight - viewport.clientHeight - viewport.scrollTop;
      const hasOverflow = viewport.scrollHeight > viewport.clientHeight + 24;
      const nearBottom = hiddenHeight <= 96;
      isNearBottomRef.current = nearBottom;
      setShowScrollToBottom(hasOverflow && !nearBottom);
    };

    updateScrollState();
    viewport.addEventListener("scroll", updateScrollState, { passive: true });
    window.addEventListener("resize", updateScrollState);

    return () => {
      viewport.removeEventListener("scroll", updateScrollState);
      window.removeEventListener("resize", updateScrollState);
    };
  }, [
    isStandaloneWorkspace,
    selectedConversationId,
    selectedConversationTurns.length,
    selectedConversationLastTurnKey,
  ]);

  useLayoutEffect(() => {
    if (selectedConversation) {
      return;
    }

    const frames: number[] = [];
    resetWorkspaceScrollTop();
    frames.push(window.requestAnimationFrame(resetWorkspaceScrollTop));
    frames.push(window.requestAnimationFrame(() => {
      frames.push(window.requestAnimationFrame(resetWorkspaceScrollTop));
    }));
    return () => {
      frames.forEach((frame) => window.cancelAnimationFrame(frame));
    };
  }, [resetWorkspaceScrollTop, selectedConversation]);

  useEffect(() => {
    const conversationChanged =
      previousSelectedConversationIdRef.current !== selectedConversationId;
    const turnCountIncreased =
      selectedConversationTurns.length > previousTurnCountRef.current;
    const lastTurnChanged =
      previousLastTurnKeyRef.current !== selectedConversationLastTurnKey;

    previousSelectedConversationIdRef.current = selectedConversationId;
    previousTurnCountRef.current = selectedConversationTurns.length;
    previousLastTurnKeyRef.current = selectedConversationLastTurnKey;

    if (!selectedConversation && processingConversationIds.size === 0) {
      return;
    }

    if (
      !conversationChanged &&
      !turnCountIncreased &&
      !(lastTurnChanged && isNearBottomRef.current)
    ) {
      return;
    }

    const behavior =
      conversationChanged || !isNearBottomRef.current ? "auto" : "smooth";
    return scheduleScrollToBottom(behavior, conversationChanged ? 1800 : 900);
  }, [
    processingConversationIds.size,
    scheduleScrollToBottom,
    selectedConversation,
    selectedConversationId,
    selectedConversationLastTurnKey,
    selectedConversationTurns.length,
  ]);

  useEffect(() => {
    if (!isStandaloneWorkspace || !selectedConversationId) {
      return;
    }

    return scheduleScrollToBottom("auto", 1800);
  }, [isStandaloneWorkspace, scheduleScrollToBottom, selectedConversationId]);

  useEffect(() => {
    if (activeRequestStartedAt === null) {
      setSubmitElapsedSeconds(0);
      return;
    }

    const updateElapsed = () => {
      setSubmitElapsedSeconds(
        Math.max(0, Math.floor((Date.now() - activeRequestStartedAt) / 1000)),
      );
    };

    updateElapsed();
    const timer = window.setInterval(updateElapsed, 1000);
    return () => {
      window.clearInterval(timer);
    };
  }, [activeRequestStartedAt]);

  useEffect(() => {
    const textarea = textareaRef.current;
    if (!textarea) {
      return;
    }

    textarea.style.height = "auto";
    const maxHeight = Math.min(
      480,
      Math.max(260, Math.floor(window.innerHeight * 0.42)),
    );
    textarea.style.height = `${Math.min(textarea.scrollHeight, maxHeight)}px`;
  }, [imagePrompt, mode]);

  useEffect(() => {
    window.dispatchEvent(
      new CustomEvent("image-studio:mobile-workspace-title", {
        detail: { title: selectedConversation?.title ?? null },
      }),
    );
  }, [selectedConversation?.title]);

  const persistConversation = useCallback(
    async (conversation: ImageConversation) => {
      const normalizedConversation = normalizeConversation(conversation);
      if (mountedRef.current) {
        draftSelectionRef.current = false;
        setSelectedConversationId(normalizedConversation.id);
        setConversations((prev) => {
          const next = [
            normalizedConversation,
            ...prev.filter((item) => item.id !== normalizedConversation.id),
          ].sort((a, b) => b.createdAt.localeCompare(a.createdAt));
          return next;
        });
      }
      await saveImageConversation(normalizedConversation);
    },
    [setConversations, setSelectedConversationId],
  );

  const updateConversation = useCallback(
    async (
      conversationId: string,
      updater: (current: ImageConversation | null) => ImageConversation,
    ) => {
      if (mountedRef.current) {
        setConversations((prev) => {
          const currentConversation =
            prev.find((item) => item.id === conversationId) ?? null;
          const optimisticConversation = normalizeConversation(
            updater(currentConversation),
          );
          const next = [
            optimisticConversation,
            ...prev.filter((item) => item.id !== conversationId),
          ].sort((a, b) => b.createdAt.localeCompare(a.createdAt));
          return next;
        });
      }

      const nextConversation = await updateImageConversation(
        conversationId,
        updater,
      );
      if (!mountedRef.current) {
        return;
      }
      setConversations((prev) => {
        const next = [
          nextConversation,
          ...prev.filter((item) => item.id !== conversationId),
        ].sort((a, b) => b.createdAt.localeCompare(a.createdAt));
        return next;
      });
    },
    [setConversations],
  );

  const handleRenameConversation = useCallback(
    async (conversationId: string, title: string) => {
      const trimmedTitle = title.trim();
      if (!trimmedTitle) {
        toast.error("请输入对话名");
        return;
      }
      try {
        const renamedConversation = await renameImageConversation(
          conversationId,
          trimmedTitle,
        );
        if (!mountedRef.current) {
          return;
        }
        setConversations((prev) =>
          [
            renamedConversation,
            ...prev.filter((item) => item.id !== conversationId),
          ].sort((a, b) => b.createdAt.localeCompare(a.createdAt)),
        );
        toast.success("对话名已更新");
      } catch (error) {
        const message = error instanceof Error ? error.message : "重命名失败";
        toast.error(message);
        throw error;
      }
    },
    [setConversations],
  );

  const resetComposer = useCallback(
    (nextMode: ImageMode = mode) => {
      setMode(nextMode);
      setImagePrompt("");
      setImageCount("1");
      setSourceImages([]);
      setComposerResetKey((value) => value + 1);
    },
    [mode, setSourceImages],
  );

  const openWorkspaceView = useCallback(() => {
    navigate("/image/workspace");
  }, [navigate]);

  const handleCreateDraftAndOpenWorkspace = useCallback(() => {
    handleCreateDraft(resetComposer, textareaRef);
    openWorkspaceView();
  }, [handleCreateDraft, openWorkspaceView, resetComposer]);

  const handleFocusConversationAndOpenWorkspace = useCallback(
    (conversationId: string) => {
      focusConversation(conversationId);
      void refreshConversation(conversationId, { silent: true });
      openWorkspaceView();
    },
    [focusConversation, openWorkspaceView, refreshConversation],
  );

  const handleCancelTurn = useCallback(
    async (conversationId: string, turn: ImageConversationTurn) => {
      if (!turn.jobId) {
        toast.error("无法定位要取消的任务");
        return;
      }
      await updateConversation(conversationId, (current) => {
        const base = current ?? selectedConversation;
        return {
          ...(base ?? {
            id: conversationId,
            title: "新建生图会话",
            mode: "generate" as const,
            prompt: turn.prompt,
            model: turn.model,
            count: turn.count,
            images: turn.images,
            createdAt: turn.createdAt,
            status: turn.status,
            turns: [turn],
          }),
          turns: (base?.turns ?? [turn]).map((item) =>
            item.id === turn.id
              ? {
                  ...item,
                  status: "cancelled",
                  cancelRequested: true,
                  waitingDetail: "cancel_requested",
                  finishedAt: new Date().toISOString(),
                  images: item.images.map((image) =>
                    image.status === "loading"
                      ? {
                          ...image,
                          status: "error",
                          error: "本次生成已取消",
                        }
                      : image,
                  ),
                }
              : item,
          ),
        };
      });
      try {
        await cancelBusinessImageJob(turn.jobId);
        toast.success("已请求取消任务");
      } catch (error) {
        const message = error instanceof Error ? error.message : "取消任务失败";
        toast.error(message);
      } finally {
        void refreshConversation(conversationId, { silent: true });
      }
    },
    [refreshConversation, selectedConversation, updateConversation],
  );

  const applyPromptExample = useCallback(
    (example: (typeof inspirationExamples)[number]) => {
      setMode("generate");
      setImageCount(String(example.count));
      setImagePrompt(example.prompt);
      openDraftConversation();
      setSourceImages([]);
      textareaRef.current?.focus();
    },
    [openDraftConversation, setSourceImages],
  );

  const { handleSelectionEditSubmit, handleRetryTurn, handleSubmit } =
    useImageSubmit({
      mode,
      imagePrompt,
      imageModel: "gpt-image-2",
      imageSources,
      sourceImages,
      parsedCount,
      imageSize,
      imageResolutionAccess,
      imageQuality,
      providerPlatform,
      selectedConversationId,
      editorTarget,
      makeId,
      focusConversation,
      closeSelectionEditor,
      setImagePrompt,
      setSourceImages,
      setSubmitElapsedSeconds,
      persistConversation,
      updateConversation,
      resetComposer,
    });

  const composer = (
    <PromptComposer
      mode={mode}
      modeOptions={modeOptions}
      imageCount={imageCount}
      imageAspectRatio={imageAspectRatio}
      imageAspectRatioOptions={imageAspectRatioOptions}
      imageResolutionTier={imageResolutionTier}
      imageResolutionTierLabel={imageResolutionTierLabel}
      imageResolutionTierOptions={imageResolutionTierOptions}
      imageSizeHint={imageSizeHint}
      providerPlatform={providerPlatform}
      providerPlatformOptions={providerPlatformOptions}
      imageQuality={imageQuality}
      imageQualityOptions={imageQualityOptions}
      imageQualityDisabled={!isImageQualityEnabled}
      imageQualityDisabledReason={imageQualityDisabledReason}
      sourceImages={sourceImages}
      imagePrompt={imagePrompt}
      textareaRef={textareaRef}
      uploadInputRef={uploadInputRef}
      maskInputRef={maskInputRef}
      onModeChange={setMode}
      onImageCountChange={setImageCount}
      onImageAspectRatioChange={(value) =>
        setImageAspectRatio(value as ImageAspectRatio)
      }
      onImageResolutionTierChange={(value) =>
        setImageResolutionTier(value as ImageResolutionTier)
      }
      onProviderPlatformChange={setProviderPlatform}
      onImageQualityChange={(value) => setImageQuality(value as ImageQuality)}
      onPromptChange={setImagePrompt}
      onPromptPaste={handlePromptPaste}
      onRemoveSourceImage={removeSourceImage}
      onOpenSourceSelectionEditor={openSourceSelectionEditor}
      onAppendFiles={appendFiles}
      onMobileCollapsedChange={setIsMobileComposerCollapsed}
      composerResetKey={composerResetKey}
      placement={selectedConversation ? "bottom" : "inline"}
      onSubmit={handleSubmit}
    />
  );

  const historyPanel = (
    <HistorySidebar
      conversations={displayedConversations}
      selectedConversationId={selectedConversationId}
      isLoadingHistory={isLoadingHistory}
      hasProcessingConversations={processingConversationIds.size > 0}
      processingConversationIds={processingConversationIds}
      modeLabelMap={modeLabelMap}
      buildConversationPreviewSource={buildConversationPreviewSource}
      formatConversationTime={formatConversationTime}
      onCreateDraft={handleCreateDraftAndOpenWorkspace}
      onClearHistory={handleClearHistory}
      onFocusConversation={handleFocusConversationAndOpenWorkspace}
      onRenameConversation={handleRenameConversation}
      onDeleteConversation={handleDeleteConversation}
      onCollapse={
        !isStandaloneHistory && !isStandaloneWorkspace
          ? () => setHistoryCollapsed(true)
          : undefined
      }
      standalone={isStandaloneHistory}
    />
  );

  const workspacePanel = (
    <div
      data-image-workspace-panel
      className={cn(
        "relative flex h-full min-h-0 min-w-0 flex-col overflow-hidden bg-transparent",
        !isStandaloneWorkspace && "lg:min-h-0",
      )}
    >
      <WorkspaceHeader
        selectedConversationTitle={
          selectedConversation
            ? formatImageConversationTitle(
                selectedConversation.title,
                selectedConversation.prompt,
              )
            : null
        }
      />

      {historyCollapsed && !isStandaloneWorkspace && !isStandaloneHistory ? (
        <button
          type="button"
          data-history-expand
          onClick={() => setHistoryCollapsed(false)}
          className="absolute left-4 top-5 z-20 inline-flex h-10 items-center gap-2 rounded-lg border border-[var(--app-border)] bg-[var(--app-bg-surface)] px-3 text-[13px] font-semibold text-[var(--app-text-secondary)] shadow-none backdrop-blur transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)] sm:left-[50px] sm:top-[39px]"
          aria-label="展开历史对话"
          title="展开历史对话"
        >
          <PanelLeftOpen className="size-4" />
          <span className="hidden sm:inline">历史</span>
        </button>
      ) : null}

      <div
        className={cn(
          "relative min-h-0 flex-1 bg-transparent",
        )}
      >
        <div
          ref={resultsViewportRef}
          className={cn(
            "hide-scrollbar min-h-[240px] overflow-visible lg:h-full lg:min-h-0 lg:overflow-y-auto lg:pb-0",
            isMobileComposerCollapsed
              ? "pb-[180px] sm:pb-[190px]"
              : "pb-[248px] sm:pb-[264px]",
          )}
        >
          <div ref={resultsContentRef}>
            {!selectedConversation ? (
              <EmptyState
                inspirationExamples={inspirationExamples}
                composer={composer}
                onApplyPromptExample={applyPromptExample}
              />
            ) : (
              <ConversationTurns
                conversationId={selectedConversation.id}
                turns={selectedConversationTurns}
                modeLabelMap={modeLabelMap}
                activeRequest={activeRequest}
                processingStatus={processingStatus}
                waitingDots={waitingDots}
                submitElapsedSeconds={submitElapsedSeconds}
                formatConversationTime={formatConversationTime}
                formatProcessingDuration={formatProcessingDuration}
                onOpenSelectionEditor={openSelectionEditor}
                onRetryTurn={handleRetryTurn}
                onCancelTurn={handleCancelTurn}
              />
            )}
            <div
              ref={bottomAnchorRef}
              style={{ height: COMPOSER_SAFE_BOTTOM_OFFSET }}
              aria-hidden="true"
            />
          </div>
        </div>
        {showScrollToBottom ? (
          <button
            type="button"
            onClick={() => scrollToBottom("smooth")}
            className={cn(
              "absolute right-5 z-10 inline-flex size-11 items-center justify-center rounded-full border border-[var(--app-border)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)] shadow-lg shadow-black/40 backdrop-blur transition hover:bg-[var(--app-bg-surface-hover)] lg:bottom-5",
              isMobileComposerCollapsed
                ? "bottom-[148px] sm:bottom-[158px]"
                : "bottom-[190px] sm:bottom-[204px]",
            )}
            aria-label="滚动到底部"
            title="滚动到底部"
          >
            <ChevronsDown className="size-5" />
          </button>
        ) : null}
      </div>

      {selectedConversation ? composer : null}
    </div>
  );

  return (
    <section
      className={cn(
        "grid h-full min-h-full grid-cols-1 overflow-hidden bg-transparent text-[var(--app-text-primary)]",
        historyCollapsed || isStandaloneWorkspace || isStandaloneHistory
          ? "lg:grid-cols-[minmax(0,1fr)]"
          : "lg:grid-cols-[276px_minmax(0,1fr)]",
      )}
    >
      {!isStandaloneWorkspace && !historyCollapsed ? historyPanel : null}
      {isStandaloneHistory ? (
        <div className="min-h-0 overflow-y-auto bg-transparent p-4">
          {historyPanel}
        </div>
      ) : null}
      {!isStandaloneHistory ? workspacePanel : null}

      <ImageEditModal
        key={editorTarget?.imageName || "image-edit-modal"}
        open={Boolean(editorTarget)}
        imageName={editorTarget?.imageName || "image.png"}
        imageSrc={editorTarget?.sourceDataUrl || ""}
        isSubmitting={false}
        allowOutputOptions={Boolean(editorTarget)}
        imageAspectRatio={imageAspectRatio}
        imageAspectRatioOptions={imageAspectRatioOptions}
        imageResolutionTier={imageResolutionTier}
        imageResolutionTierOptions={imageResolutionTierOptions}
        imageQuality={imageQuality}
        imageQualityOptions={imageQualityOptions}
        imageQualityDisabled={!isImageQualityEnabled}
        imageQualityDisabledReason={imageQualityDisabledReason}
        providerPlatform={selectionEditorProviderPlatform}
        providerPlatformOptions={providerPlatformOptions}
        onImageAspectRatioChange={(value) =>
          setImageAspectRatio(value as ImageAspectRatio)
        }
        onImageResolutionTierChange={(value) =>
          setImageResolutionTier(value as ImageResolutionTier)
        }
        onProviderPlatformChange={setSelectionEditorProviderPlatform}
        onImageQualityChange={(value) => setImageQuality(value as ImageQuality)}
        onClose={closeSelectionEditor}
        onSubmit={async (payload) => {
          await handleSelectionEditSubmit(payload);
        }}
      />
    </section>
  );
}
