import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../../utils";

const buttonVariants = cva("border bg-transparent cursor-pointer px-4 py-0.5", {
  variants: {
    variant: { default: "" },
    size: { default: "" },
  },
  defaultVariants: {
    variant: "default",
    size: "default",
  },
});

const Button = ({
  className,
  variant = "default",
  size = "default",
  ...props
}: React.ComponentProps<"button"> & VariantProps<typeof buttonVariants>) => {
  return (
    <button
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
};

export default Button;
