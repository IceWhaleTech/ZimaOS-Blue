// Portuguese - Portugal (Português de Portugal)
import ptBR from './pt-BR'

export default {
  ...ptBR,
  common: {
    ...ptBR.common,
    // Portuguese (Portugal) variations
    save: 'Guardar', // instead of 'Salvar'
    cancel: 'Cancelar',
    delete: 'Eliminar', // instead of 'Excluir'
    create: 'Criar',
    search: 'Pesquisar',
    settings: 'Definições', // instead of 'Configurações'
    language: 'Idioma',
    theme: 'Tema',
    refresh: 'Actualizar', // Portuguese (PT) spelling
    refreshing: 'A actualizar...',
  },
  auth: {
    ...ptBR.auth,
    signIn: 'Iniciar sessão', // instead of 'Entrar'
    signOut: 'Terminar sessão', // instead of 'Sair'
    signInToAccount: 'Inicie sessão na sua conta',
    signingIn: 'A iniciar sessão...',
    signInSuccessful: 'Sessão iniciada com sucesso!',
    signInFailed: 'Falha ao iniciar sessão',
    completingSignIn: 'A completar início de sessão...',
    verifyingCredentials: 'Aguarde enquanto verificamos as suas credenciais',
    redirecting: 'A redireccionar para a aplicação...',
    backToLogin: 'Voltar ao início de sessão',
    username: 'Nome de utilizador', // instead of 'Nome de usuário'
    password: 'Palavra-passe', // instead of 'Senha'
    enterUsername: 'Introduza o seu nome de utilizador',
    enterPassword: 'Introduza a sua palavra-passe',
    rememberMe: 'Lembrar-me',
    forgotPassword: 'Esqueceu-se da palavra-passe?',
    orContinueWith: 'Ou continuar com',
    signInWith: 'Iniciar sessão com {provider}',
  },
  nav: {
    ...ptBR.nav,
    settings: 'Definições',
    chat: 'Conversa',
    dashboard: 'Painel',
    plugins: 'Extensões',
    profile: 'Perfil',
    channels: 'Canais',
    automation: 'Automação',
    security: 'Segurança',
  },
  localeNames: {
    ...ptBR.localeNames,
    'pt-PT': 'Português (Portugal)',
    'pt-BR': 'Português (Brasil)',
  },
}
