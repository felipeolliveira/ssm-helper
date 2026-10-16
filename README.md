# ssm-helper

Script para Windows que abre túneis do **AWS Systems Manager (SSM)** usando login via **AWS SSO (IAM Identity Center)**. Ele roda dentro do WSL (Ubuntu) e tem um menu interativo no terminal.

Com ele você acessa recursos em redes privadas da AWS (load balancers internos, bancos RDS etc.) pela sua máquina local, sem VPN e sem bastion com SSH.

## Pré-requisitos

- Windows 10/11 com **WSL** instalado
- Uma distribuição **Ubuntu 20.04 ou superior** no WSL
- Acesso ao AWS SSO, com permissão para as contas e roles configuradas

O script instala sozinho as dependências que faltarem no Ubuntu. Para isso, ele pede a senha do `sudo` do Linux:

- [AWS CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- [Session Manager Plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)
- [yq](https://github.com/mikefarah/yq)

## Instalação

1. Clone o repositório ou baixe os arquivos.
2. Copie `config.yaml.example` para `config.yaml`, na mesma pasta do `.bat`.
3. Preencha o `config.yaml` com os dados reais (veja [Configuração](#configuração)).
4. Dê dois cliques em `ssm-helper.bat`.

> O `config.yaml` está no `.gitignore` e **não deve ser versionado**.

## Configuração

O `config.yaml` tem três seções:

```yaml
variables:
  MENU_TITLE: "Ambiente de QA"

profiles:
  qa-admin:
    AWS_SSO_URL: "https://identitycenter.amazonaws.com/ssoins-0000000000000000"
    AWS_SSO_REGION: "us-east-1"
    AWS_SSO_ACCOUNT_ID: "111111111111"
    AWS_SSO_ROLE_NAME: "NomeDaRole"

targets:
  qa-web:
    SSM_TARGET: "i-0123456789abcdef0"
    SSM_REGION: "us-east-1"
    SSM_DOCUMENT: "AWS-StartPortForwardingSessionToRemoteHost"
    SSM_REMOTE_HOST: "internal-meu-alb-qa-0000000000.us-east-1.elb.amazonaws.com"
    SSM_REMOTE_PORT: "443"
    SSM_LOCAL_PORT: "443"
    AVAILABLE_LINKS: "qa.example.com api.qa.example.com"
```

### `variables`

| Campo | Obrigatório | Descrição |
|---|---|---|
| `MENU_TITLE` | Sim | Título exibido no menu principal |

### `profiles`

Cada chave é o nome de um perfil AWS. O nome aceita só letras, números, `.`, `_` e `-`.

| Campo | Obrigatório | Descrição |
|---|---|---|
| `AWS_SSO_URL` | Sim | URL de início do IAM Identity Center |
| `AWS_SSO_REGION` | Sim | Região do Identity Center. Também é a região padrão do perfil |
| `AWS_SSO_ACCOUNT_ID` | Sim | ID da conta AWS (12 dígitos) |
| `AWS_SSO_ROLE_NAME` | Sim | Nome da role (permission set) no SSO |

### `targets`

Cada chave é o nome de um destino de túnel.

| Campo | Obrigatório | Descrição |
|---|---|---|
| `SSM_TARGET` | Sim | ID da instância EC2 com o SSM Agent, usada como ponte |
| `SSM_REGION` | Não | Região da instância. Se ficar vazio, usa a `AWS_SSO_REGION` do perfil escolhido |
| `SSM_DOCUMENT` | Sim | Documento SSM, normalmente `AWS-StartPortForwardingSessionToRemoteHost` |
| `SSM_REMOTE_HOST` | Sim | Host de destino, acessível a partir da instância |
| `SSM_REMOTE_PORT` | Sim | Porta no host de destino |
| `SSM_LOCAL_PORT` | Sim | Porta local exposta na sua máquina |
| `AVAILABLE_LINKS` | Não | Links exibidos ao conectar, separados por espaço |

O script valida o arquivo ao iniciar e lista os campos ausentes ou inválidos, se houver.

## Uso

Ao abrir o `.bat`, o menu principal mostra:

```
1) Conectar ao ambiente (Iniciar túnel SSM)
2) Login SSO / Atualizar sessão AWS
3) Visualizar perfis AWS
4) Visualizar targets
0) Sair
```

- **Conectar**: você escolhe o perfil e o target. Se a sessão SSO estiver expirada, o login é renovado automaticamente pelo navegador do Windows. O túnel fica aberto até você apertar `Ctrl+C`.
- **Login SSO**: faz o login de um perfil sem abrir túnel.
- **Visualizar perfis / targets**: mostra o que está no `config.yaml`. Para alterar, edite o arquivo.

### Portas abaixo de 1024

Se `SSM_LOCAL_PORT` for menor que 1024 (por exemplo, `443`), o túnel roda com `sudo` e pode pedir a senha do Linux.

### Acessar pelo navegador com o hostname original

Para abrir a aplicação pelo domínio real em vez de `localhost`, por exemplo por causa de certificados TLS ou CORS, aponte os domínios para `127.0.0.1` no arquivo `C:\Windows\System32\drivers\etc\hosts` do Windows:

```
127.0.0.1 qa.example.com
127.0.0.1 api.qa.example.com
```

## Como os perfis AWS são tratados

Os perfis vêm **apenas** do `config.yaml`. A cada execução, o script:

- gera um arquivo dedicado do AWS CLI em `~/.aws/ssm-helper-config`, dentro do WSL;
- aponta `AWS_CONFIG_FILE` e `AWS_SHARED_CREDENTIALS_FILE` para arquivos próprios;
- ignora `AWS_PROFILE` e as credenciais definidas em variáveis de ambiente.

Por isso, os perfis do seu `~/.aws/config` e `~/.aws/credentials` não interferem e não são alterados. O cache de login SSO (`~/.aws/sso/cache`) continua compartilhado: perfis com a mesma `AWS_SSO_URL` aproveitam o mesmo login.

## Solução de problemas

| Sintoma | Causa provável |
|---|---|
| "Nenhuma distribuição Ubuntu do WSL foi encontrada" | Instale o Ubuntu com `wsl --install -d Ubuntu` |
| "Arquivo de configuração não encontrado" | O `config.yaml` não está na mesma pasta do `.bat` |
| "O arquivo não é um YAML válido" | Use `chave: valor` (dois pontos), não `chave=valor` |
| "Falha ao verificar acesso AWS (STS)" | Você não tem acesso à role/conta configurada no perfil |
| O navegador não abre no login SSO | Copie o link exibido no terminal e abra manualmente no Windows |
| Erro de porta em uso ao conectar | Outro processo usa `SSM_LOCAL_PORT`. Encerre-o ou mude a porta |
