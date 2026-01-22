import { Tooltip } from "@mui/joy";
import { InboxIcon, LoaderIcon, TicketIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useState } from "react";
import toast from "react-hot-toast";
import { activityServiceClient } from "@/grpcweb";
import useAsyncEffect from "@/hooks/useAsyncEffect";
import useNavigateTo from "@/hooks/useNavigateTo";
import { activityNamePrefix } from "@/store/common";
import { userStore } from "@/store/v2";
import { Inbox, Inbox_Status } from "@/types/proto/api/v1/inbox_service";
import { User } from "@/types/proto/api/v1/user_service";
import { cn } from "@/utils";
import { useTranslate } from "@/utils/i18n";

interface Props {
    inbox: Inbox;
}

interface Ticket {
    id: number;
    title: string;
}

const TicketCommentMessage = observer(({ inbox }: Props) => {
    const t = useTranslate();
    const navigateTo = useNavigateTo();
    const [ticket, setTicket] = useState<Ticket | undefined>(undefined);
    const [sender, setSender] = useState<User | undefined>(undefined);
    const [initialized, setInitialized] = useState<boolean>(false);

    useAsyncEffect(async () => {
        if (!inbox.activityId) {
            return;
        }

        try {
            const activity = await activityServiceClient.getActivity({
                name: `${activityNamePrefix}${inbox.activityId}`,
            });
            if (activity.payload?.ticketComment) {
                const ticketId = activity.payload.ticketComment.ticketId;
                try {
                    const listResp = await fetch(`/api/v1/tickets`);
                    if (listResp.ok) {
                        const tickets = await listResp.json();
                        const found = tickets.find((t: any) => t.id === ticketId);
                        if (found) {
                            setTicket(found);
                        } else {
                            setTicket({ id: ticketId, title: "Unknown Ticket" });
                        }
                    }
                } catch (e) {
                    console.error("Failed to fetch ticket", e);
                    setTicket({ id: ticketId, title: "Ticket" });
                }

                const sender = await userStore.getOrFetchUserByName(inbox.sender);
                setSender(sender);
            }
        } catch (error) {
            console.error("Failed to load ticket comment activity:", error);
        } finally {
            setInitialized(true);
        }
    }, [inbox.activityId]);

    const handleNavigateToTicket = async () => {
        if (!ticket) {
            return;
        }

        navigateTo(`/tickets?id=${ticket.id}`);
        if (inbox.status === Inbox_Status.UNREAD) {
            handleArchiveMessage(true);
        }
    };

    const handleArchiveMessage = async (silence = false) => {
        await userStore.updateInbox(
            {
                name: inbox.name,
                status: Inbox_Status.ARCHIVED,
            },
            ["status"],
        );
        if (!silence) {
            toast.success(t("message.archived-successfully"));
        }
    };

    return (
        <div className="w-full flex flex-row justify-start items-start gap-3">
            <div
                className={cn(
                    "shrink-0 mt-2 p-2 rounded-full border",
                    inbox.status === Inbox_Status.UNREAD
                        ? "border-blue-600 text-blue-600 bg-blue-50 dark:bg-zinc-800"
                        : "border-gray-500 text-gray-500 bg-gray-50 dark:bg-zinc-800",
                )}
            >
                <Tooltip title={"Ticket Mention"} placement="bottom">
                    <TicketIcon className="w-4 sm:w-5 h-auto" />
                </Tooltip>
            </div>
            <div
                className={cn(
                    "border w-full p-2 px-3 rounded-lg flex flex-col justify-start items-start gap-1 dark:border-zinc-700 hover:bg-gray-100 dark:hover:bg-zinc-700",
                    inbox.status !== Inbox_Status.UNREAD && "opacity-60",
                )}
            >
                {initialized ? (
                    <>
                        <div className="w-full flex flex-row justify-between items-center">
                            <span className="text-sm text-gray-500">{inbox.createTime?.toLocaleString()}</span>
                            <div>
                                {inbox.status === Inbox_Status.UNREAD && (
                                    <Tooltip title={t("common.archive")} placement="top">
                                        <InboxIcon
                                            className="w-4 h-auto cursor-pointer text-gray-400 hover:text-blue-600"
                                            onClick={() => handleArchiveMessage()}
                                        />
                                    </Tooltip>
                                )}
                            </div>
                        </div>
                        <p
                            className="text-base leading-tight cursor-pointer text-gray-500 dark:text-gray-400 hover:underline hover:text-blue-600"
                            onClick={handleNavigateToTicket}
                        >
                            {sender?.nickname || sender?.username} mentioned you in <b>Ticket #{ticket?.id}: {ticket?.title}</b>
                        </p>
                    </>
                ) : (
                    <div className="w-full flex flex-row justify-center items-center my-2">
                        <LoaderIcon className="animate-spin text-zinc-500" />
                    </div>
                )}
            </div>
        </div>
    );
});

export default TicketCommentMessage;
