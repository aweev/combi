import { useMutation } from "@connectrpc/connect-query";
import { useRouter } from "@/i18n/routing";

import { sendOTP, verifyOTP } from "@gen/combi/auth/v1/auth-AuthService_connectquery";
import { OtpChannel, SendOTPResponseSchema, VerifyOTPResponseSchema } from "@gen/combi/auth/v1/auth_pb";
import type { DescMethodUnary } from "@bufbuild/protobuf";
import { SendOTPRequestSchema, VerifyOTPRequestSchema } from "@gen/combi/auth/v1/auth_pb";

export function useAuth() {
    const router = useRouter();

    const sendOtpMutation = useMutation(
        sendOTP as DescMethodUnary<typeof SendOTPRequestSchema, typeof SendOTPResponseSchema>
    );

    const verifyOtpMutation = useMutation(
        verifyOTP as DescMethodUnary<typeof VerifyOTPRequestSchema, typeof VerifyOTPResponseSchema>,
        {
            onSuccess: (data) => {
                if (data.sessionTokens?.accessToken) {
                    localStorage.setItem("cast_jwt", data.sessionTokens.accessToken);

                    if (data.recycledSimSuspect) {
                        console.warn("Recycled SIM suspected. Flagging verification status.");
                    }

                    router.push("/app/home");
                }
            },
        }
    );

    const requestVerificationCode = async (phone: string, deviceHash: string) => {
        await sendOtpMutation.mutateAsync({
            phone,
            deviceHash,
            preferredChannel: OtpChannel.WHATSAPP,
        });
    };

    const submitVerificationCode = async (challengeId: string, code: string, deviceHash: string) => {
        await verifyOtpMutation.mutateAsync({
            challengeId,
            code,
            deviceHash,
        });
    };

    return {
        requestVerificationCode,
        submitVerificationCode,
        isSending: sendOtpMutation.isPending,
        isVerifying: verifyOtpMutation.isPending,
        challengeId: sendOtpMutation.data?.challengeId,
        error: sendOtpMutation.error || verifyOtpMutation.error,
    };
}