<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:if test="1=1">
			<item>
				<xsl:value-of select="/root/item"/>
			</item>
		</xsl:if>
	</xsl:template>
</xsl:stylesheet>