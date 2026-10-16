@echo off
title Ambiente de QA
echo =======================================================
echo Iniciando ambiente WSL (Ubuntu)...
echo =======================================================

rem Check if WSL is installed and working
wsl --status >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERRO] O WSL nao parece estar instalado ou configurado neste computador.
    echo Por favor, instale o WSL com Ubuntu ^(20.04 ou superior^) antes de continuar.
    pause
    exit /b
)

rem Identify Ubuntu distribution using PowerShell with Regex
for /f "delims=" %%I in ('powershell -NoProfile -Command "$lines = (wsl -l) -replace '\x00', ''; $def = $lines | Where-Object { $_ -match '\*\s*.*Ubuntu.*' }; if ($def) { 'DEFAULT' } else { $ub = (wsl -l -q) -replace '\x00', '' | Where-Object { $_ -match 'Ubuntu' } | Select-Object -First 1; if ($ub) { $ub.Trim() } else { 'NONE' } }"') do set "WSL_DISTRO=%%I"

rem Validate if there is any Ubuntu
if "%WSL_DISTRO%"=="NONE" (
    echo [ERRO] Nenhuma distribuicao Ubuntu do WSL foi encontrada neste computador.
    echo O script requer o Ubuntu ^(versao 20.04 ou superior^) para instalar dependencias como o AWS CLI e o Session Manager Plugin.
    pause
    exit /b
)

rem Configure WSL command and display name
if "%WSL_DISTRO%"=="DEFAULT" (
    set "WSL_CMD=wsl"
    set "DISTRO_NAME=Padrao do Sistema (Ubuntu)"
) else (
    set "WSL_CMD=wsl -d %WSL_DISTRO%"
    set "DISTRO_NAME=%WSL_DISTRO%"
)

rem Logs
echo --- Distribuicoes WSL Encontradas ---
wsl --list
echo -------------------------------------
for /F %%a in ('echo prompt $E ^| cmd') do set "ESC=%%a"
echo %ESC%[32m[INFO] %DISTRO_NAME% selecionado!%ESC%[0m
echo.
echo Preparando para iniciar o shell. Aguarde...
timeout /t 2 /nobreak >nul

rem Extract bash section, save to temp file and run (pipe breaks interactive read)
%WSL_CMD% bash -c "export CONFIG_FILE='$(wslpath '%~dp0config.yaml')'; sed -n '/^#BASH_START/,\$p' '$(wslpath '%~f0')' | sed 's/\r$//' > /tmp/acesso_ssm_qa.sh && bash /tmp/acesso_ssm_qa.sh; rm -f /tmp/acesso_ssm_qa.sh"

echo.
echo Sessao finalizada.
pause
exit /b

#BASH_START
#!/bin/bash

# Terminal colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

# AWS paths (fixed)
# Profiles come only from config.yaml: they are written to a dedicated AWS CLI
# config file, so profiles in ~/.aws/config and ~/.aws/credentials are ignored.
AWS_DIR="$HOME/.aws"
export AWS_CONFIG_FILE="$AWS_DIR/acesso-ssm-qa-config"
export AWS_SHARED_CREDENTIALS_FILE="$AWS_DIR/acesso-ssm-qa-credentials"
unset AWS_PROFILE AWS_DEFAULT_PROFILE AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN

# Dependency installation (fixed)
INSTALL_DIR="/tmp/acesso-ssm-qa-install"
AWS_CLI_URL="https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip"
AWS_CLI_ZIP="awscliv2.zip"
SSM_PLUGIN_URL="https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_64bit/session-manager-plugin.deb"
SSM_PLUGIN_DEB="session-manager-plugin.deb"
YQ_URL="https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64"
YQ_BIN="yq"

# TUI labels (fixed)
PROFILE_MENU_TITLE="Escolha o Perfil AWS"
TARGET_MENU_TITLE="Escolha o Target"
MENU_SEPARATOR="==================================="
MENU_OPTION_CONNECT="1) Conectar ao ambiente (Iniciar túnel SSM)"
MENU_OPTION_CONFIGURE="2) Login SSO / Atualizar sessão AWS"
MENU_OPTION_VIEW_PROFILES="3) Visualizar perfis AWS"
MENU_OPTION_VIEW_TARGETS="4) Visualizar targets"
MENU_OPTION_EXIT="0) Sair"
MENU_OPTION_BACK="0) Voltar"

# Prompts (TUI)
PROMPT_OPTION="Escolha uma opção: "
PROMPT_BACK_TO_MENU="Pressione Enter para voltar ao menu..."
PROMPT_CONTINUE="Pressione Enter para continuar..."

# Messages (TUI)
MSG_AWS_CLI_INSTALLING="AWS CLI não encontrado. Instalando..."
MSG_SSM_PLUGIN_INSTALLING="Session Manager Plugin não encontrado. Instalando..."
MSG_YQ_INSTALLING="yq não encontrado. Instalando..."
MSG_DEPS_INSTALLED="AWS CLI, Session Manager Plugin e yq instalados com sucesso"
MSG_PROFILE_CONFIGURED="Perfil '%s' autenticado via SSO com sucesso!"
PROFILE_VIEW_TITLE="Perfis AWS configurados"
MSG_PROFILES_EDIT_HINT="Para adicionar, remover ou alterar perfis, edite o arquivo config.yaml (mesma pasta do .bat)."
PROFILE_VIEW_NAME="  Nome: %s"
PROFILE_VIEW_ACCOUNT="  Conta: %s"
PROFILE_VIEW_ROLE="  Role: %s"
PROFILE_VIEW_REGION="  Região: %s"
MSG_SSO_VERIFYING="Verificando sessão AWS, aguarde..."
MSG_STS_VERIFY_FAILED="Falha ao verificar acesso AWS (STS). Confirme com o TechLead se você possui a role '%s'."
MSG_SSO_LOGIN_MANUAL="Não foi possível abrir o navegador. Copie o link abaixo e abra no Windows:"
MSG_SSO_LOGIN_FAILED="Falha no login SSO."
MSG_CONFIG_NOT_FOUND="Arquivo de configuração não encontrado."
MSG_CONFIG_NOT_FOUND_HINT="Copie config.example.yaml para config.yaml e preencha os valores."
MSG_CONFIG_INCOMPLETE="Configuração incompleta. Corrija o arquivo:"
MSG_CONFIG_ERRORS_LIST="Problemas encontrados:"
MSG_CONFIG_INVALID_YAML="O arquivo não é um YAML válido."
MSG_CONFIG_YAML_HINT="Use o formato 'chave: valor' (dois pontos), não '=' como em .conf. Veja config.example.yaml."
MSG_SSO_REFRESHING="Sessão expirada. Renovando via SSO..."
MSG_INVALID_OPTION="Opção inválida! Tente novamente."
MSG_EXITING="Saindo..."
TARGETS_VIEW_TITLE="Targets configurados"
MSG_TARGETS_EDIT_HINT="Para adicionar, remover ou alterar targets, edite o arquivo config.yaml (mesma pasta do .bat)."
TARGET_VIEW_NAME="  Nome: %s"
TARGET_VIEW_INSTANCE="  Instância (SSM_TARGET): %s"
TARGET_VIEW_DOCUMENT="  Documento: %s"
TARGET_VIEW_REMOTE="  Host remoto: %s"
TARGET_VIEW_PORTS="  Portas (remota → local): %s → %s"
TARGET_VIEW_REGION="  Região: %s"
TARGET_VIEW_REGION_DEFAULT="(região do perfil)"
SSM_CONNECT_TITLE="Conexão SSM"
SSM_CONNECT_PROFILE="  Perfil: %s"
SSM_CONNECT_TARGET="  Target: %s"
SSM_CONNECT_PORT="  Porta local: %s"
SSM_CONNECT_REGION="  Região: %s"
SSM_CONNECT_SUDO="  • Pode ser necessário sudo (senha Linux) para a porta %s"
SSM_CONNECT_STOP="  • Ctrl+C encerra o túnel"
SSM_CONNECT_LINKS_TITLE="  Links disponíveis:"
SSM_CONNECT_LINK_ITEM="    • %s"
EXTERNAL_TITLE_SSO="Login SSO — AWS CLI"
EXTERNAL_TITLE_SSM="Túnel SSM — Session Manager"
MSG_SSO_PROCESS_HINT="O navegador do Windows será aberto. Aguarde e conclua a autenticação."
MSG_EXTERNAL_HINT="As mensagens abaixo são geradas automaticamente pela AWS."
MSG_SSM_PROCESS_HINT="Processo em execução. Logs em inglês abaixo são das ferramentas AWS CLI e Session Manager Plugin."
MSG_SSM_FAILED="Falha ao iniciar o túnel SSM."
MSG_EXTERNAL_END="--- Processo AWS finalizado ---"
EXTERNAL_SEPARATOR="-----------------------------------"

yq_get_variable() {
    yq -r ".variables.${1} // \"\"" "$CONFIG_FILE"
}

yq_get_profile_field() {
    local profile_name="$1"
    local field_name="$2"
    yq -r ".profiles.\"${profile_name}\".${field_name} // \"\"" "$CONFIG_FILE"
}

yq_get_target_field() {
    local target_name="$1"
    local field_name="$2"
    yq -r ".targets.\"${target_name}\".${field_name} // \"\"" "$CONFIG_FILE"
}

is_config_value_empty() {
    local value="$1"
    [ -z "$value" ] || [ "$value" = "null" ]
}

config_yaml_section_type() {
    yq -r ".${1} | type" "$CONFIG_FILE" 2>/dev/null
}

is_config_yaml_map_section() {
    [ "$(config_yaml_section_type "$1")" = "!!map" ]
}

print_config_validation_errors() {
    echo -e "${RED}${MSG_CONFIG_INCOMPLETE}${NC}"
    echo "  $CONFIG_FILE"
    echo "${MSG_CONFIG_ERRORS_LIST}"
    while [ $# -gt 0 ]; do
        echo "  - $1"
        shift
    done
    read -p "Pressione Enter para sair..."
    exit 1
}

load_config_variables() {
    MENU_TITLE=$(yq_get_variable MENU_TITLE)
}

load_profile_sso_settings() {
    local profile_name="$1"
    AWS_SSO_URL=$(yq_get_profile_field "$profile_name" AWS_SSO_URL)
    AWS_SSO_REGION=$(yq_get_profile_field "$profile_name" AWS_SSO_REGION)
    AWS_SSO_ACCOUNT_ID=$(yq_get_profile_field "$profile_name" AWS_SSO_ACCOUNT_ID)
    AWS_SSO_ROLE_NAME=$(yq_get_profile_field "$profile_name" AWS_SSO_ROLE_NAME)
}

load_target_ssm_settings() {
    local target_name="$1"
    SSM_TARGET=$(yq_get_target_field "$target_name" SSM_TARGET)
    SSM_REGION=$(yq_get_target_field "$target_name" SSM_REGION)
    SSM_DOCUMENT=$(yq_get_target_field "$target_name" SSM_DOCUMENT)
    SSM_REMOTE_HOST=$(yq_get_target_field "$target_name" SSM_REMOTE_HOST)
    SSM_REMOTE_PORT=$(yq_get_target_field "$target_name" SSM_REMOTE_PORT)
    SSM_LOCAL_PORT=$(yq_get_target_field "$target_name" SSM_LOCAL_PORT)
    AVAILABLE_LINKS=$(yq_get_target_field "$target_name" AVAILABLE_LINKS)
    SSM_PARAMETERS="{\"host\":[\"${SSM_REMOTE_HOST}\"],\"portNumber\":[\"${SSM_REMOTE_PORT}\"],\"localPortNumber\":[\"${SSM_LOCAL_PORT}\"]}"
}

# Uses the target region, falling back to the profile's SSO region
resolve_ssm_region() {
    local profile_name="$1"

    if is_config_value_empty "$SSM_REGION"; then
        load_profile_sso_settings "$profile_name"
        SSM_REGION="$AWS_SSO_REGION"
    fi
}

print_available_links() {
    local links="$1"
    local link

    if is_config_value_empty "$links"; then
        return
    fi

    echo -e "${CYAN}${SSM_CONNECT_LINKS_TITLE}${NC}"
    for link in $links; do
        if [[ "$link" == http://* || "$link" == https://* ]]; then
            printf "${CYAN}${SSM_CONNECT_LINK_ITEM}${NC}\n" "$link"
        else
            printf "${CYAN}${SSM_CONNECT_LINK_ITEM}${NC}\n" "https://${link}"
        fi
    done
}

list_config_profiles() {
    mapfile -t profiles < <(yq -r '.profiles | keys | .[]' "$CONFIG_FILE" 2>/dev/null)
}

list_config_targets() {
    mapfile -t targets < <(yq -r '.targets | keys | .[]' "$CONFIG_FILE" 2>/dev/null)
}

# Rewrites the dedicated AWS CLI config with the profiles from config.yaml
write_aws_profiles_config() {
    local tmp_file
    local profile_name

    mkdir -p "$AWS_DIR"
    tmp_file=$(mktemp)
    for profile_name in "${profiles[@]}"; do
        load_profile_sso_settings "$profile_name"
        {
            echo "[profile ${profile_name}]"
            echo "sso_start_url = ${AWS_SSO_URL}"
            echo "sso_region = ${AWS_SSO_REGION}"
            echo "sso_account_id = ${AWS_SSO_ACCOUNT_ID}"
            echo "sso_role_name = ${AWS_SSO_ROLE_NAME}"
            echo "region = ${AWS_SSO_REGION}"
            echo ""
        } >> "$tmp_file"
    done
    mv "$tmp_file" "$AWS_CONFIG_FILE"
}

load_and_validate_config() {
    if [ -z "$CONFIG_FILE" ] || [ ! -f "$CONFIG_FILE" ]; then
        echo -e "${RED}${MSG_CONFIG_NOT_FOUND}${NC}"
        echo "${MSG_CONFIG_NOT_FOUND_HINT}"
        echo "Caminho esperado: config.yaml (mesma pasta do .bat)"
        read -p "Pressione Enter para sair..."
        exit 1
    fi

    local yq_parse_err=""
    if ! yq_parse_err=$(yq -e '.' "$CONFIG_FILE" 2>&1 >/dev/null); then
        echo -e "${RED}${MSG_CONFIG_INCOMPLETE}${NC}"
        echo "  $CONFIG_FILE"
        echo -e "${YELLOW}${MSG_CONFIG_INVALID_YAML}${NC}"
        echo "  ${yq_parse_err#Error: }"
        echo ""
        echo "  ${MSG_CONFIG_YAML_HINT}"
        read -p "Pressione Enter para sair..."
        exit 1
    fi

    local missing=()
    local var
    local required_vars=(
        MENU_TITLE
    )
    local required_profile_fields=(
        AWS_SSO_URL
        AWS_SSO_REGION
        AWS_SSO_ACCOUNT_ID
        AWS_SSO_ROLE_NAME
    )
    local required_target_fields=(
        SSM_TARGET
        SSM_DOCUMENT
        SSM_REMOTE_HOST
        SSM_REMOTE_PORT
        SSM_LOCAL_PORT
    )

    if is_config_yaml_map_section variables; then
        for var in "${required_vars[@]}"; do
            if is_config_value_empty "$(yq_get_variable "$var")"; then
                missing+=("variables.${var}")
            fi
        done
    else
        missing+=("variables (seção ausente)")
    fi

    if is_config_yaml_map_section profiles; then
        list_config_profiles
        if [ ${#profiles[@]} -eq 0 ]; then
            missing+=("profiles (nenhum perfil definido)")
        else
            local profile_name
            local field
            for profile_name in "${profiles[@]}"; do
                if ! [[ "$profile_name" =~ ^[A-Za-z0-9._-]+$ ]]; then
                    missing+=("profiles.${profile_name} (nome inválido: use apenas letras, números, '.', '_' ou '-')")
                    continue
                fi
                for field in "${required_profile_fields[@]}"; do
                    if is_config_value_empty "$(yq_get_profile_field "$profile_name" "$field")"; then
                        missing+=("profiles.${profile_name}.${field}")
                    fi
                done
            done
        fi
    else
        missing+=("profiles (seção ausente)")
    fi

    if is_config_yaml_map_section targets; then
        list_config_targets
        if [ ${#targets[@]} -eq 0 ]; then
            missing+=("targets (nenhum target definido)")
        else
            local target_name
            local field
            for target_name in "${targets[@]}"; do
                for field in "${required_target_fields[@]}"; do
                    if is_config_value_empty "$(yq_get_target_field "$target_name" "$field")"; then
                        missing+=("targets.${target_name}.${field}")
                    fi
                done
            done
        fi
    else
        missing+=("targets (seção ausente)")
    fi

    if [ ${#missing[@]} -gt 0 ]; then
        print_config_validation_errors "${missing[@]}"
    fi

    load_config_variables
    write_aws_profiles_config
}

# 1. Dependency validation
DEPS_INSTALLED=false

mkdir -p "$INSTALL_DIR"

if ! command -v aws &> /dev/null; then
    echo -e "${YELLOW}${MSG_AWS_CLI_INSTALLING}${NC}"
    sudo apt-get update -y && sudo apt-get install -y unzip curl
    curl -fsSL "$AWS_CLI_URL" -o "$INSTALL_DIR/$AWS_CLI_ZIP"
    unzip -q "$INSTALL_DIR/$AWS_CLI_ZIP" -d "$INSTALL_DIR"
    sudo "$INSTALL_DIR/aws/install"
    rm -rf "$INSTALL_DIR"
    mkdir -p "$INSTALL_DIR"
    DEPS_INSTALLED=true
fi

if ! command -v session-manager-plugin &> /dev/null; then
    echo -e "${YELLOW}${MSG_SSM_PLUGIN_INSTALLING}${NC}"
    curl -fsSL "$SSM_PLUGIN_URL" -o "$INSTALL_DIR/$SSM_PLUGIN_DEB"
    sudo dpkg -i "$INSTALL_DIR/$SSM_PLUGIN_DEB"
    rm -rf "$INSTALL_DIR"
    mkdir -p "$INSTALL_DIR"
    DEPS_INSTALLED=true
fi

if ! command -v yq &> /dev/null; then
    echo -e "${YELLOW}${MSG_YQ_INSTALLING}${NC}"
    curl -fsSL "$YQ_URL" -o "$INSTALL_DIR/$YQ_BIN"
    sudo install -m 0755 "$INSTALL_DIR/$YQ_BIN" /usr/local/bin/yq
    rm -rf "$INSTALL_DIR"
    DEPS_INSTALLED=true
fi

rmdir "$INSTALL_DIR" 2>/dev/null || true

load_and_validate_config

if [ "$DEPS_INSTALLED" = true ]; then
    clear
    echo -e "${GREEN}${MSG_DEPS_INSTALLED}${NC}"
    echo ""
fi

# 2. TUI functions
print_external_section_start() {
    local title="$1"
    local hint="$2"
    echo ""
    echo -e "${CYAN}${EXTERNAL_SEPARATOR}${NC}"
    echo -e "${CYAN}  ${title}${NC}"
    echo -e "${CYAN}  ${hint}${NC}"
    echo -e "${CYAN}${EXTERNAL_SEPARATOR}${NC}"
    echo ""
}

print_external_section_end() {
    echo ""
    echo -e "${CYAN}${MSG_EXTERNAL_END}${NC}"
    echo ""
}

run_aws_process() {
    local title="$1"
    local hint="$2"
    shift 2

    print_external_section_start "$title" "$hint"
    "$@"
    local exit_code=$?
    print_external_section_end
    return $exit_code
}

needs_sudo_for_local_port() {
    [ "$1" -lt 1024 ]
}

run_ssm_tunnel() {
    local profile_name="$1"
    local aws_cmd=(
        aws ssm start-session
        --profile "$profile_name"
        --region "$SSM_REGION"
        --target "$SSM_TARGET"
        --document-name "$SSM_DOCUMENT"
        --parameters "$SSM_PARAMETERS"
    )

    if needs_sudo_for_local_port "$SSM_LOCAL_PORT"; then
        sudo \
            HOME="$HOME" \
            USER="$USER" \
            AWS_CONFIG_FILE="$AWS_CONFIG_FILE" \
            AWS_SHARED_CREDENTIALS_FILE="$AWS_SHARED_CREDENTIALS_FILE" \
            AWS_PROFILE= \
            "${aws_cmd[@]}"
    else
        "${aws_cmd[@]}"
    fi
}

setup_wsl_browser() {
    local browser_helper="/tmp/access-qa-ssm-open-browser.sh"

    if ! grep -qi microsoft /proc/version 2>/dev/null; then
        return
    fi

    cat > "$browser_helper" << 'EOF'
#!/bin/bash
/mnt/c/Windows/System32/cmd.exe /c start "" "$1" >/dev/null 2>&1
EOF
    chmod +x "$browser_helper"
    export BROWSER="$browser_helper"
}

is_wsl() {
    grep -qi microsoft /proc/version 2>/dev/null
}

sso_login() {
    local profile_name="$1"

    if is_wsl; then
        setup_wsl_browser
        if aws sso login --profile "$profile_name"; then
            return 0
        fi
        echo -e "${YELLOW}${MSG_SSO_LOGIN_MANUAL}${NC}"
        aws sso login --profile "$profile_name" --no-browser
        return $?
    fi

    aws sso login --profile "$profile_name"
}

verify_sts_caller_identity() {
    local profile_name="$1"
    local sts_output
    local sts_exit

    sts_output=$(aws sts get-caller-identity --profile "$profile_name" 2>&1)
    sts_exit=$?

    if [ $sts_exit -eq 0 ]; then
        return 0
    fi

    load_profile_sso_settings "$profile_name"
    echo ""
    printf "${RED}${MSG_STS_VERIFY_FAILED}${NC}\n" "$AWS_SSO_ROLE_NAME"
    echo ""
    echo "$sts_output"
    return 1
}

ensure_sso_session() {
    local profile_name="$1"

    if aws sts get-caller-identity --profile "$profile_name" &>/dev/null; then
        return 0
    fi

    echo -e "${YELLOW}${MSG_SSO_REFRESHING}${NC}"
    run_aws_process "$EXTERNAL_TITLE_SSO" "$MSG_SSO_PROCESS_HINT" sso_login "$profile_name" || return 1
    verify_sts_caller_identity "$profile_name"
}

# Sets selected_profile from config.yaml; returns 1 when the user goes back
select_config_profile() {
    local profile_option

    while true; do
        clear
        echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
        echo -e "${CYAN}        ${PROFILE_MENU_TITLE}        ${NC}"
        echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
        for i in "${!profiles[@]}"; do
            echo "$((i + 1))) ${profiles[$i]}"
        done
        echo "$MENU_OPTION_BACK"
        echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
        read -p "$PROMPT_OPTION" profile_option
        profile_option="${profile_option//$'\r'/}"

        if [ "$profile_option" = "0" ]; then
            return 1
        fi

        if ! [[ "$profile_option" =~ ^[0-9]+$ ]] || [ "$profile_option" -lt 1 ] || [ "$profile_option" -gt ${#profiles[@]} ]; then
            echo -e "${RED}${MSG_INVALID_OPTION}${NC}"
            read -p "$PROMPT_BACK_TO_MENU"
            continue
        fi

        selected_profile="${profiles[$((profile_option - 1))]}"
        return 0
    done
}

view_sso_profiles() {
    clear
    echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
    echo -e "${CYAN}        ${PROFILE_VIEW_TITLE}        ${NC}"
    echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
    echo ""
    echo -e "${YELLOW}${MSG_PROFILES_EDIT_HINT}${NC}"
    echo ""

    local profile_name
    local idx
    for idx in "${!profiles[@]}"; do
        profile_name="${profiles[$idx]}"
        load_profile_sso_settings "$profile_name"
        echo -e "${CYAN}--- Perfil $((idx + 1)) ---${NC}"
        printf "${PROFILE_VIEW_NAME}\n" "$profile_name"
        printf "${PROFILE_VIEW_ACCOUNT}\n" "$AWS_SSO_ACCOUNT_ID"
        printf "${PROFILE_VIEW_ROLE}\n" "$AWS_SSO_ROLE_NAME"
        printf "${PROFILE_VIEW_REGION}\n" "$AWS_SSO_REGION"
        echo ""
    done

    echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
    read -p "$PROMPT_BACK_TO_MENU"
}

configure_sso() {
    if ! select_config_profile; then
        return
    fi

    if ! run_aws_process "$EXTERNAL_TITLE_SSO" "$MSG_SSO_PROCESS_HINT" sso_login "$selected_profile"; then
        echo -e "${RED}${MSG_SSO_LOGIN_FAILED}${NC}"
        read -p "$PROMPT_BACK_TO_MENU"
        return
    fi

    echo -e "${CYAN}${MSG_SSO_VERIFYING}${NC}"
    if verify_sts_caller_identity "$selected_profile"; then
        printf "${GREEN}${MSG_PROFILE_CONFIGURED}${NC}\n" "$selected_profile"
    fi
    read -p "$PROMPT_BACK_TO_MENU"
}

view_targets() {
    clear
    list_config_targets

    echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
    echo -e "${CYAN}        ${TARGETS_VIEW_TITLE}        ${NC}"
    echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
    echo ""
    echo -e "${YELLOW}${MSG_TARGETS_EDIT_HINT}${NC}"
    echo ""

    if [ ${#targets[@]} -eq 0 ]; then
        echo -e "${YELLOW}  (nenhum target definido em config.yaml)${NC}"
    else
        local target_name
        local idx
        for idx in "${!targets[@]}"; do
            target_name="${targets[$idx]}"
            load_target_ssm_settings "$target_name"
            echo -e "${CYAN}--- Target $((idx + 1)) ---${NC}"
            printf "${TARGET_VIEW_NAME}\n" "$target_name"
            printf "${TARGET_VIEW_INSTANCE}\n" "$SSM_TARGET"
            if is_config_value_empty "$SSM_REGION"; then
                printf "${TARGET_VIEW_REGION}\n" "$TARGET_VIEW_REGION_DEFAULT"
            else
                printf "${TARGET_VIEW_REGION}\n" "$SSM_REGION"
            fi
            printf "${TARGET_VIEW_DOCUMENT}\n" "$SSM_DOCUMENT"
            printf "${TARGET_VIEW_REMOTE}\n" "$SSM_REMOTE_HOST"
            printf "${TARGET_VIEW_PORTS}\n" "$SSM_REMOTE_PORT" "$SSM_LOCAL_PORT"
            echo ""
        done
    fi

    echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
    read -p "$PROMPT_BACK_TO_MENU"
}

connect_ssm() {
    list_config_targets

    while true; do
        if ! select_config_profile; then
            return
        fi

        echo -e "${CYAN}${MSG_SSO_VERIFYING}${NC}"
        if ! ensure_sso_session "$selected_profile"; then
            read -p "$PROMPT_BACK_TO_MENU"
            continue
        fi

        while true; do
            clear
            echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
            echo -e "${CYAN}        ${TARGET_MENU_TITLE}        ${NC}"
            echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
            for i in "${!targets[@]}"; do
                echo "$((i + 1))) ${targets[$i]}"
            done
            echo "$MENU_OPTION_BACK"
            echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
            read -p "$PROMPT_OPTION" target_option
            target_option="${target_option//$'\r'/}"

            if [ "$target_option" = "0" ]; then
                break
            fi

            if ! [[ "$target_option" =~ ^[0-9]+$ ]] || [ "$target_option" -lt 1 ] || [ "$target_option" -gt ${#targets[@]} ]; then
                echo -e "${RED}${MSG_INVALID_OPTION}${NC}"
                read -p "$PROMPT_BACK_TO_MENU"
                continue
            fi

            selected_target="${targets[$((target_option - 1))]}"
            load_target_ssm_settings "$selected_target"
            resolve_ssm_region "$selected_profile"

            clear
            echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
            echo -e "${CYAN}        ${SSM_CONNECT_TITLE}        ${NC}"
            echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
            printf "${CYAN}${SSM_CONNECT_PROFILE}${NC}\n" "$selected_profile"
            printf "${CYAN}${SSM_CONNECT_TARGET}${NC}\n" "$selected_target"
            printf "${CYAN}${SSM_CONNECT_REGION}${NC}\n" "$SSM_REGION"
            printf "${CYAN}${SSM_CONNECT_PORT}${NC}\n" "$SSM_LOCAL_PORT"
            if needs_sudo_for_local_port "$SSM_LOCAL_PORT"; then
                printf "${CYAN}${SSM_CONNECT_SUDO}${NC}\n" "$SSM_LOCAL_PORT"
            fi
            echo -e "${CYAN}${SSM_CONNECT_STOP}${NC}"
            print_available_links "$AVAILABLE_LINKS"
            echo -e "${CYAN}${MENU_SEPARATOR}${NC}"

            if ! run_aws_process "$EXTERNAL_TITLE_SSM" "$MSG_SSM_PROCESS_HINT" \
                run_ssm_tunnel "$selected_profile"; then
                echo -e "${RED}${MSG_SSM_FAILED}${NC}"
                read -p "$PROMPT_BACK_TO_MENU"
            fi
            return
        done
    done
}

# 3. Main menu
while true; do
    clear
    echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
    echo -e "${CYAN}        ${MENU_TITLE}         ${NC}"
    echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
    echo "$MENU_OPTION_CONNECT"
    echo "$MENU_OPTION_CONFIGURE"
    echo "$MENU_OPTION_VIEW_PROFILES"
    echo "$MENU_OPTION_VIEW_TARGETS"
    echo "$MENU_OPTION_EXIT"
    echo -e "${CYAN}${MENU_SEPARATOR}${NC}"
    read -p "$PROMPT_OPTION" option
    option="${option//$'\r'/}"

    case $option in
        1)
            connect_ssm
            ;;
        2)
            configure_sso
            ;;
        3)
            view_sso_profiles
            ;;
        4)
            view_targets
            ;;
        0)
            echo -e "${GREEN}${MSG_EXITING}${NC}"
            break
            ;;
        *)
            echo -e "${RED}${MSG_INVALID_OPTION}${NC}"
            read -p "$PROMPT_CONTINUE"
            ;;
    esac
done

