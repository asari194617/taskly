module.exports = async function isRepoWriter(github, repo, user) {
  if (user?.type !== 'User') return false;
  try {
    const { data } = await github.rest.repos.getCollaboratorPermissionLevel({
      ...repo, username: user.login,
    });
    return data.user?.id === user.id && ['admin', 'write'].includes(data.permission);
  } catch (error) {
    if (error.status === 404) return false;
    throw error;
  }
};
