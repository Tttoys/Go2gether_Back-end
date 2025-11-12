#!/bin/bash

# Notification API cURL Commands
# Usage: ./notification_curl.sh [command] [args...]
# 
# Commands:
#   list [limit] [offset] [unread_only] [type]  - List notifications
#   read [notification_id]                       - Mark notification as read
#   read-all                                    - Mark all as read

BASE_URL="http://localhost:8080"
JWT_TOKEN="${JWT_TOKEN:-YOUR_JWT_TOKEN}"

if [ -z "$JWT_TOKEN" ] || [ "$JWT_TOKEN" == "YOUR_JWT_TOKEN" ]; then
    echo "Error: Please set JWT_TOKEN environment variable"
    echo "Example: export JWT_TOKEN='your-token-here'"
    exit 1
fi

case "$1" in
    list)
        LIMIT="${2:-20}"
        OFFSET="${3:-0}"
        UNREAD_ONLY="${4:-false}"
        TYPE="${5:-}"
        
        URL="${BASE_URL}/api/notifications?limit=${LIMIT}&offset=${OFFSET}&unread_only=${UNREAD_ONLY}"
        if [ -n "$TYPE" ]; then
            URL="${URL}&type=${TYPE}"
        fi
        
        echo "=== Listing notifications ==="
        curl -X GET "$URL" \
            -H "Authorization: Bearer ${JWT_TOKEN}" \
            -H "Content-Type: application/json" | jq .
        ;;
    
    read)
        if [ -z "$2" ]; then
            echo "Error: Notification ID required"
            echo "Usage: $0 read <notification_id>"
            exit 1
        fi
        
        NOTIFICATION_ID="$2"
        echo "=== Marking notification as read ==="
        curl -X POST "${BASE_URL}/api/notifications/${NOTIFICATION_ID}/read" \
            -H "Authorization: Bearer ${JWT_TOKEN}" \
            -H "Content-Type: application/json" | jq .
        ;;
    
    read-all)
        echo "=== Marking all notifications as read ==="
        curl -X POST "${BASE_URL}/api/notifications/read-all" \
            -H "Authorization: Bearer ${JWT_TOKEN}" \
            -H "Content-Type: application/json" | jq .
        ;;
    
    *)
        echo "Notification API cURL Commands"
        echo ""
        echo "Usage: $0 [command] [args...]"
        echo ""
        echo "Commands:"
        echo "  list [limit] [offset] [unread_only] [type]  - List notifications"
        echo "    Example: $0 list 20 0 false member_joined"
        echo ""
        echo "  read <notification_id>                       - Mark notification as read"
        echo "    Example: $0 read 509c0de9-162b-4dca-89c4-dda12ce5ec61"
        echo ""
        echo "  read-all                                    - Mark all as read"
        echo "    Example: $0 read-all"
        echo ""
        echo "Environment Variables:"
        echo "  JWT_TOKEN - Your JWT authentication token (required)"
        echo "  BASE_URL  - API base URL (default: http://localhost:8080)"
        echo ""
        echo "Examples:"
        echo "  export JWT_TOKEN='your-token'"
        echo "  $0 list"
        echo "  $0 list 50 0 true"
        echo "  $0 list 20 0 false trip_update"
        echo "  $0 read 509c0de9-162b-4dca-89c4-dda12ce5ec61"
        echo "  $0 read-all"
        exit 1
        ;;
esac

