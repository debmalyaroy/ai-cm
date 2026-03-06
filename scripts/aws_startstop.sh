#!/bin/bash
# =============================================================================
# AI-CM: AWS EC2 + RDS Start / Stop Helper
# Manages your EC2 instance and RDS database to stay within Free Tier limits.
#
# Prerequisites:
#   - AWS CLI v2 installed and configured (aws configure)
#   - EC2_INSTANCE_ID and RDS_INSTANCE_ID set in [prod.aws] section of root .env
#
# Usage:
#   ./scripts/aws_startstop.sh start   — Start EC2 and RDS
#   ./scripts/aws_startstop.sh stop    — Stop EC2 and RDS
#   ./scripts/aws_startstop.sh status  — Show current state of both
# =============================================================================

set -e

# ── Load IDs from [prod.aws] section of root .env ────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
ENV_FILE="$ROOT_DIR/.env"

if [ -f "$ENV_FILE" ]; then
    in_section=0
    while IFS= read -r line || [ -n "$line" ]; do
        [[ "$line" =~ ^#.* ]] && continue
        [[ -z "${line// }" ]] && continue
        if [[ "$line" =~ ^\[(.*)\]$ ]]; then
            if [ "${BASH_REMATCH[1]}" == "prod.aws" ]; then in_section=1; else in_section=0; fi
            continue
        fi
        if [ $in_section -eq 1 ] && [[ "$line" =~ ^([^=]+)=(.*)$ ]]; then
            key="${BASH_REMATCH[1]// /}"
            value="${BASH_REMATCH[2]%%#*}"
            value="${value%"${value##*[![:space:]]}"}"
            value="${value%\"}"; value="${value#\"}"; value="${value%\'}"; value="${value#\'}"
            case "$key" in
                EC2_INSTANCE_ID) [ -z "$EC2_INSTANCE_ID" ] && EC2_INSTANCE_ID="$value" ;;
                RDS_INSTANCE_ID) [ -z "$RDS_INSTANCE_ID" ] && RDS_INSTANCE_ID="$value" ;;
                AWS_REGION)      [ -z "$AWS_REGION" ]      && AWS_REGION="$value" ;;
            esac
        fi
    done < "$ENV_FILE"
fi

AWS_REGION="${AWS_REGION:-us-east-1}"
ACTION="${1:-status}"

if [ -z "$EC2_INSTANCE_ID" ] || [ -z "$RDS_INSTANCE_ID" ]; then
    echo "EC2_INSTANCE_ID and RDS_INSTANCE_ID must be set."
    echo "   Add them to the [prod.aws] section of your root .env file:"
    echo "     EC2_INSTANCE_ID=i-0123456789abcdef0"
    echo "     RDS_INSTANCE_ID=aicm-postgres"
    exit 1
fi

ec2_state() {
    aws ec2 describe-instances \
        --instance-ids "$EC2_INSTANCE_ID" \
        --region "$AWS_REGION" \
        --query "Reservations[0].Instances[0].State.Name" \
        --output text 2>/dev/null || echo "unknown"
}

rds_state() {
    aws rds describe-db-instances \
        --db-instance-identifier "$RDS_INSTANCE_ID" \
        --region "$AWS_REGION" \
        --query "DBInstances[0].DBInstanceStatus" \
        --output text 2>/dev/null || echo "unknown"
}

case "$ACTION" in
# ── STATUS ────────────────────────────────────────────────────────────────────
status)
    EC2_S=$(ec2_state)
    RDS_S=$(rds_state)
    echo "──────────────────────────────────────"
    echo " EC2  ($EC2_INSTANCE_ID): $EC2_S"
    echo " RDS  ($RDS_INSTANCE_ID): $RDS_S"
    echo "──────────────────────────────────────"
    ;;

# ── START ─────────────────────────────────────────────────────────────────────
start)
    echo "Starting RDS instance: $RDS_INSTANCE_ID..."
    RDS_S=$(rds_state)
    if [ "$RDS_S" = "available" ]; then
        echo "   RDS is already running."
    elif [ "$RDS_S" = "stopped" ]; then
        aws rds start-db-instance \
            --db-instance-identifier "$RDS_INSTANCE_ID" \
            --region "$AWS_REGION" > /dev/null
        echo "   Waiting for RDS to become available (may take 2-4 min)..."
        aws rds wait db-instance-available \
            --db-instance-identifier "$RDS_INSTANCE_ID" \
            --region "$AWS_REGION"
        echo "   RDS is available."
    else
        echo "   RDS state is '$RDS_S' — cannot start. Check AWS Console."
        exit 1
    fi

    echo "Starting EC2 instance: $EC2_INSTANCE_ID..."
    EC2_S=$(ec2_state)
    if [ "$EC2_S" = "running" ]; then
        echo "   EC2 is already running."
    elif [ "$EC2_S" = "stopped" ]; then
        aws ec2 start-instances \
            --instance-ids "$EC2_INSTANCE_ID" \
            --region "$AWS_REGION" > /dev/null
        echo "   Waiting for EC2 to reach running state..."
        aws ec2 wait instance-running \
            --instance-ids "$EC2_INSTANCE_ID" \
            --region "$AWS_REGION"
        echo "   EC2 is running."
    else
        echo "   EC2 state is '$EC2_S' — cannot start. Check AWS Console."
        exit 1
    fi

    PUBLIC_IP=$(aws ec2 describe-instances \
        --instance-ids "$EC2_INSTANCE_ID" \
        --region "$AWS_REGION" \
        --query "Reservations[0].Instances[0].PublicIpAddress" \
        --output text 2>/dev/null)
    echo ""
    echo "Services started."
    echo "   App URL: http://$PUBLIC_IP"
    echo "   SSH:     ssh -i your-key.pem ec2-user@$PUBLIC_IP"
    echo ""
    echo "WARNING: Stop services when not in use to stay within Free Tier:"
    echo "   ./scripts/aws_startstop.sh stop"
    ;;

# ── STOP ──────────────────────────────────────────────────────────────────────
stop)
    echo "Stopping EC2 instance: $EC2_INSTANCE_ID..."
    EC2_S=$(ec2_state)
    if [ "$EC2_S" = "stopped" ]; then
        echo "   EC2 is already stopped."
    elif [ "$EC2_S" = "running" ]; then
        aws ec2 stop-instances \
            --instance-ids "$EC2_INSTANCE_ID" \
            --region "$AWS_REGION" > /dev/null
        echo "   Waiting for EC2 to stop..."
        aws ec2 wait instance-stopped \
            --instance-ids "$EC2_INSTANCE_ID" \
            --region "$AWS_REGION"
        echo "   EC2 stopped."
    else
        echo "   EC2 state is '$EC2_S' — stopping may not be possible right now."
    fi

    echo "Stopping RDS instance: $RDS_INSTANCE_ID..."
    RDS_S=$(rds_state)
    if [ "$RDS_S" = "stopped" ]; then
        echo "   RDS is already stopped."
    elif [ "$RDS_S" = "available" ]; then
        aws rds stop-db-instance \
            --db-instance-identifier "$RDS_INSTANCE_ID" \
            --region "$AWS_REGION" > /dev/null
        echo "   RDS stop initiated (takes ~1 min; no need to wait)."
        echo "   RDS stopping."
    else
        echo "   RDS state is '$RDS_S' — cannot stop right now."
    fi

    echo ""
    echo "Stop commands issued."
    echo "   EC2 and RDS are not billed while stopped."
    echo "   Note: RDS auto-starts after 7 days (AWS limitation)."
    ;;

*)
    echo "Usage: $0 [start|stop|status]"
    exit 1
    ;;
esac
