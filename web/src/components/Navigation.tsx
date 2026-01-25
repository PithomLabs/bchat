import { Tooltip } from "@mui/joy";
import { BellIcon, BotIcon, DatabaseIcon, EarthIcon, FlaskConicalIcon, LibraryIcon, MessageSquareIcon, PaperclipIcon, SettingsIcon, TicketIcon, UserCircleIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useState } from "react";
import { NavLink } from "react-router-dom";
import useCurrentUser from "@/hooks/useCurrentUser";
import { Routes } from "@/router";
import { agentAdminStore, userStore } from "@/store/v2";
import { cn } from "@/utils";
import { useTranslate } from "@/utils/i18n";
import { isSuperUser } from "@/utils/user";
import BrandBanner from "./BrandBanner";
import UserBanner from "./UserBanner";

interface NavLinkItem {
  id: string;
  path: string;
  title: string;
  icon: React.ReactNode;
}

interface Props {
  collapsed?: boolean;
  className?: string;
}

const Navigation = observer((props: Props) => {
  const { collapsed, className } = props;
  const t = useTranslate();
  const currentUser = useCurrentUser();
  const isAdmin = isSuperUser(currentUser);
  const [permissionsLoaded, setPermissionsLoaded] = useState(false);

  useEffect(() => {
    if (!currentUser) {
      return;
    }

    userStore.fetchInboxes();
    userStore.fetchNotifications();
    userStore.listenToNotifications();

    // Fetch user's tenant permissions for non-admin users (to determine Internal Agent visibility)
    if (!isAdmin) {
      agentAdminStore.fetchUserTenants().then(() => {
        setPermissionsLoaded(true);
      });
    } else {
      // Admins don't need to wait for permissions
      setPermissionsLoaded(true);
    }
  }, [currentUser, isAdmin]);

  const homeNavLink: NavLinkItem = {
    id: "header-memos",
    path: Routes.ROOT,
    title: t("common.memos"),
    icon: <LibraryIcon className="w-6 h-auto opacity-70 shrink-0" />,
  };
  const exploreNavLink: NavLinkItem = {
    id: "header-explore",
    path: Routes.EXPLORE,
    title: t("common.explore"),
    icon: <EarthIcon className="w-6 h-auto opacity-70 shrink-0" />,
  };
  const resourcesNavLink: NavLinkItem = {
    id: "header-resources",
    path: Routes.RESOURCES,
    title: t("common.resources"),
    icon: <PaperclipIcon className="w-6 h-auto opacity-70 shrink-0" />,
  };
  const ticketsNavLink: NavLinkItem = {
    id: "header-tickets",
    path: Routes.TICKETS,
    title: "Tickets",
    icon: <TicketIcon className="w-6 h-auto opacity-70 shrink-0" />,
  };

  const unreadCount = userStore.state.unreadNotificationCount;

  const notificationsNavLink: NavLinkItem = {
    id: "header-notifications",
    path: Routes.NOTIFICATIONS,
    title: "Notifications",
    icon: (
      <div className="relative">
        <BellIcon className="w-6 h-auto opacity-70 shrink-0" />
        {unreadCount > 0 && (
          <span className="absolute -top-1 -right-1 flex h-4 w-4 items-center justify-center rounded-full bg-red-500 text-[10px] text-white">
            {unreadCount > 9 ? "9+" : unreadCount}
          </span>
        )}
      </div>
    ),
  };
  const chatNavLink: NavLinkItem = {
    id: "header-chat",
    path: Routes.CHAT,
    title: t("common.chat"),
    icon: <MessageSquareIcon className="w-6 h-auto opacity-70 shrink-0" />,
  };
  const internalAgentNavLink: NavLinkItem = {
    id: "header-internal-agent",
    path: Routes.INTERNAL_AGENT,
    title: t("common.internal-agent"),
    icon: <BotIcon className="w-6 h-auto opacity-70 shrink-0" />,
  };
  const agentSimulationNavLink: NavLinkItem = {
    id: "header-agent-simulation",
    path: Routes.AGENT_SIMULATION,
    title: t("common.agent-simulation"),
    icon: <FlaskConicalIcon className="w-6 h-auto opacity-70 shrink-0" />,
  };
  const agentAdminNavLink: NavLinkItem = {
    id: "header-agent-admin",
    path: Routes.AGENT_ADMIN,
    title: t("common.agent-admin"),
    icon: <SettingsIcon className="w-6 h-auto opacity-70 shrink-0" />,
  };
  const ragStatsNavLink: NavLinkItem = {
    id: "header-rag-stats",
    path: Routes.RAG_STATS,
    title: t("common.rag-stats"),
    icon: <DatabaseIcon className="w-6 h-auto opacity-70 shrink-0" />,
  };
  const signInNavLink: NavLinkItem = {
    id: "header-auth",
    path: Routes.AUTH,
    title: t("common.sign-in"),
    icon: <UserCircleIcon className="w-6 h-auto opacity-70 shrink-0" />,
  };

  // Internal Agent is only visible to admins or users with chat:test on at least one tenant
  // Wait for permissions to load before checking (prevents flash of hidden menu)
  const canAccessInternalAgent = isAdmin || (permissionsLoaded && agentAdminStore.hasAnyChatTestPermission());

  const baseNavLinks: NavLinkItem[] = [
    homeNavLink,
    exploreNavLink,
    resourcesNavLink,
    ticketsNavLink,
    notificationsNavLink,
    chatNavLink,
    ...(canAccessInternalAgent ? [internalAgentNavLink, agentSimulationNavLink] : []),
  ];
  const adminNavLinks: NavLinkItem[] = isAdmin ? [agentAdminNavLink, ragStatsNavLink] : [];

  const navLinks: NavLinkItem[] = currentUser
    ? [...baseNavLinks, ...adminNavLinks]
    : [exploreNavLink, signInNavLink];

  return (
    <header
      className={cn(
        "w-full h-full overflow-auto flex flex-col justify-between items-start gap-4 py-4 md:pt-6 z-30 hide-scrollbar",
        className,
      )}
    >
      <div className="w-full px-1 py-1 flex flex-col justify-start items-start space-y-2 overflow-auto hide-scrollbar shrink">
        <NavLink className="mb-2 cursor-default" to={currentUser ? Routes.ROOT : Routes.EXPLORE}>
          <BrandBanner collapsed={collapsed} />
        </NavLink>
        {navLinks.map((navLink) => (
          <NavLink
            className={({ isActive }) =>
              cn(
                "px-2 py-2 rounded-2xl border flex flex-row items-center text-lg text-gray-800 dark:text-gray-400 hover:bg-white hover:border-gray-200 dark:hover:border-zinc-700 dark:hover:bg-zinc-800",
                collapsed ? "" : "w-full px-4",
                isActive ? "bg-white drop-shadow-sm dark:bg-zinc-800 border-gray-200 dark:border-zinc-700" : "border-transparent",
              )
            }
            key={navLink.id}
            to={navLink.path}
            id={navLink.id}
            viewTransition
          >
            {props.collapsed ? (
              <Tooltip title={navLink.title} placement="right" arrow>
                <div>{navLink.icon}</div>
              </Tooltip>
            ) : (
              navLink.icon
            )}
            {!props.collapsed && <span className="ml-3 truncate">{navLink.title}</span>}
          </NavLink>
        ))}
      </div>
      {currentUser && <UserBanner collapsed={collapsed} />}
    </header>
  );
});

export default Navigation;
