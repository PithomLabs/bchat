import { Table, Button, Tooltip, IconButton } from "@mui/joy";
import { CheckIcon, BellIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect } from "react";
import { Link } from "react-router-dom";
import Empty from "@/components/Empty";
import MobileHeader from "@/components/MobileHeader";
import useCurrentUser from "@/hooks/useCurrentUser";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";
import { userStore } from "@/store/v2";
import { useTranslate } from "@/utils/i18n";

const Notifications = observer(() => {
    const t = useTranslate();
    const { md } = useResponsiveWidth();
    const notifications = userStore.state.notifications;

    const currentUser = useCurrentUser();

    useEffect(() => {
        if (currentUser) {
            userStore.fetchNotifications();
        }
    }, [currentUser]);

    const handleMarkAsRead = async (id: number) => {
        await userStore.patchNotification(id, true);
    };

    return (
        <section className="@container w-full max-w-5xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
            {!md && <MobileHeader />}
            <div className="w-full px-4 sm:px-6">
                <div className="w-full shadow flex flex-col justify-start items-start px-4 py-3 rounded-xl bg-white dark:bg-zinc-800 text-black dark:text-gray-300">
                    <div className="relative w-full flex flex-row justify-between items-center mb-4">
                        <p className="py-1 flex flex-row justify-start items-center select-none opacity-80">
                            <BellIcon className="w-6 h-auto mr-1 opacity-80" />
                            <span className="text-lg">Notifications</span>
                        </p>
                    </div>

                    {notifications.length === 0 ? (
                        <div className="w-full mt-4 mb-8 flex flex-col justify-center items-center italic">
                            <Empty />
                            <p className="mt-4 text-gray-600 dark:text-gray-400">{t("message.no-data")}</p>
                        </div>
                    ) : (
                        <Table hoverRow>
                            <thead>
                                <tr>
                                    <th style={{ width: '40%' }}>Ticket</th>
                                    <th>Date</th>
                                    <th>Status</th>
                                    <th>Action</th>
                                </tr>
                            </thead>
                            <tbody>
                                {notifications.map((notification) => (
                                    <tr key={notification.id}>
                                        <td>
                                            <Link
                                                to={notification.ticketUrl}
                                                className="text-blue-600 hover:underline"
                                                onClick={() => handleMarkAsRead(notification.id)}
                                            >
                                                View Ticket (Initiator: {notification.initiatorName})
                                            </Link>
                                        </td>
                                        <td>{new Date(notification.createdTs * 1000).toLocaleString()}</td>
                                        <td>
                                            {notification.isRead ? (
                                                <span className="text-gray-500">Read</span>
                                            ) : (
                                                <span className="text-green-600 font-bold">Unread</span>
                                            )}
                                        </td>
                                        <td>
                                            {!notification.isRead && (
                                                <Tooltip title="Mark as read">
                                                    <IconButton onClick={() => handleMarkAsRead(notification.id)}>
                                                        <CheckIcon className="w-4 h-4" />
                                                    </IconButton>
                                                </Tooltip>
                                            )}
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </Table>
                    )}
                </div>
            </div>
        </section>
    );
});

export default Notifications;
