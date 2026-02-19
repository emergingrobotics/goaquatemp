I found a relevant result - a Home Assistant integration for Aqua Temp heat pumps. Let me get more details from that GitHub repository.Let me look at the actual API implementation code to understand what endpoints the Aqua Temp app uses.I found the information. Let me get a bit more detail on the exact API structure.Based on my research, I can now give you a comprehensive answer about the Aqua Temp app's API.

The **Aqua Temp app** uses a cloud-based API provided by **Linked-Go** (also known as "Go Heating IoT Platform"). Here are the details:

## API Base URLs

There are different API versions depending on when your account was created:

1. **Older accounts (pre-v1.5.8)**: `https://cloud.linked-go.com/cloudservice/api`
2. **HiTemp accounts**: `https://cloud.linked-go.com/cloudservice/api`
3. **Newer accounts (v1.5.9+)**: `https://cloud.linked-go.com:449/crmservice/api` (with MD5-hashed passwords)

## Key API Endpoints

| Endpoint | Purpose |
|----------|---------|
| `/app/user/login.json` | Authentication - returns an x-token |
| `/app/device/deviceList.json` | List all registered devices |
| `/app/device/getDataByCode.json` | Get device sensor data/parameters |
| `/app/device/control.json` | Control the device (set temperature, power, mode) |

## Authentication

The API uses token-based authentication. You POST your credentials to the login endpoint and receive an `x-token` that must be included in the header of subsequent requests.

## Notable Characteristics

- The platform enforces **single-device login** (only one device can be logged in at a time), so many users create a secondary account for home automation integrations
- Several open-source Home Assistant integrations have reverse-engineered this API, including [radical-squared/aquatemp](https://github.com/radical-squared/aquatemp)
- The API communicates via the cloud rather than locally, meaning your heat pump data goes through Linked-Go's servers

## Setting Up a Second Account for Home Assistant Integration

### The Problem

As of app version 1.5.9, Aqua Temp / HiTemp enforces a **single login restriction**, meaning only one device can be authenticated with a given account at a time. If you log in via the mobile app, it will invalidate the Home Assistant integration's session (and vice versa), causing token expiration conflicts.

### The Solution

Create a dedicated secondary account and share your heat pump device with it. This allows:
- Your **primary account** to remain logged into the mobile app
- Your **secondary account** to be used exclusively by Home Assistant

### Step-by-Step Setup

1. **Create a new account** in the Aqua Temp mobile app using a different email address
2. **Log into your primary account** (the one that owns the device)
3. **Share the device** with your new account:
   - Navigate to your device in the app
   - Look for a "Share" or "Device Sharing" option
   - Enter the email address of your secondary account
   - Confirm the share request
4. **Accept the share** by logging into the secondary account and accepting the device share invitation
5. **Configure Home Assistant** using the secondary account credentials

### Home Assistant Integration Installation

1. In HACS, search for "Aqua Temp" and install it
2. If not found, add the custom repository `radical-squared/aquatemp` as an integration
3. Restart Home Assistant
4. Go to **Settings → Devices & Services → Add Integration**
5. Search for "Aqua Temp" and add it
6. Enter your **secondary account** credentials

### Resources

- [radical-squared/aquatemp GitHub Repository](https://github.com/radical-squared/aquatemp)
- [Home Assistant Community Thread](https://community.home-assistant.io/t/aquatemp-heat-pump-controller-integration-custom-climate-entity/334594)
