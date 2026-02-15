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


    personality: {
    ...enUS.personality,
  },
  memoryService: {
    ...enUS.memoryService,
    title: 'Serviço de memória',
    description: 'Navegar e gerir entradas de memória versionadas com isolamento por espaço de nomes',
    namespace: 'Espaço de nomes',
    allNamespaces: 'Todos os espaços de nomes',
    defaultNamespace: 'predefinido',
    createNamespace: 'Criar espaço de nomes',
    deleteNamespace: 'Eliminar espaço de nomes',
    confirmDeleteNamespace: 'Eliminar o espaço de nomes "{name}" e todas as suas entradas?',
    namespaceName: 'Nome do espaço de nomes',
    entries: 'Entradas',
    noEntries: 'Ainda sem entradas de memória',
    addEntry: 'Adicionar entrada',
    addEntryTitle: 'Adicionar entrada de memória',
    editEntry: 'Editar entrada',
    content: 'Conteúdo',
    contentPlaceholder: 'Introduzir conteúdo da memória...',
    category: 'Categoria',
    categoryPlaceholder: 'ex: preference, fact, context',
    tags: 'Etiquetas',
    tagsPlaceholder: 'etiqueta1, etiqueta2, etiqueta3',
    source: 'Fonte',
    sourcePlaceholder: 'ex: user, agent, system',
    importance: 'Importância',
    ttl: 'TTL',
    ttlPlaceholder: 'ex: 24h, 7d, 0 para sem expiração',
    contentType: 'Tipo de conteúdo',
    version: 'v{n}',
    versions: 'Versões',
    viewHistory: 'Ver histórico',
    historyTitle: 'Histórico de versões',
    status: 'Estado',
    active: 'Ativo',
    expired: 'Expirado',
    deleted: 'Eliminado',
    createdAt: 'Criado em',
    updatedAt: 'Atualizado em',
    expiresAt: 'Expira em',
    noExpiry: 'Sem expiração',
    confirmDelete: 'Eliminar esta entrada de memória?',
    purgeExpired: 'Limpar expirados',
    purgeSuccess: '{count} entradas expiradas removidas',
    stats: 'Estatísticas',
    totalEntries: 'Total',
    activeEntries: 'Ativas',
    expiredEntries: 'Expiradas',
    loadMore: 'Carregar mais',
  },
}
