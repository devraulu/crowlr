import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../../utils";
const messageVariants = cva(
  "border p-2 [&_pre]:bg-stone-200 [&_pre]:p-2 [&_pre]:overflow-x-scroll",
  {
    variants: {
      variant: {
        user: "text-right",
        system: "text-left",
        error: "",
      },
    },
  },
);

const Message = ({
  className,
  variant = "system",
  ...props
}: React.ComponentProps<"div"> & VariantProps<typeof messageVariants>) => {
  return (
    <div className={cn(messageVariants({ variant, className }))} {...props} />
  );
};

export default Message;
