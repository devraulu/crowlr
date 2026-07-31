export const err = (title: string, description: string[]) => {
  return { error: title, message: description };
};
